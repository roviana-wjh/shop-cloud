package data

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"review-service/internal/biz"
	"review-service/internal/data/model"
	"review-service/internal/data/query"
	"review-service/pkg/localcache"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/sortorder"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
)

// storeReviewListCache is the cached store review list payload.
type storeReviewListCache struct {
	Total          int64               `json:"total"`
	Reviews        []*model.ReviewInfo `json:"reviews"`
	EmptyDBChecked bool                `json:"empty_db_checked,omitempty"`
}

type goodsScoreRankMeta struct {
	SpuID       int64
	AvgScore    float64
	ReviewCount int64
}

type ReviewRepo struct {
	data  *Data
	log   *log.Helper
	local *localcache.Cache
}

const (
	goodsScoreRankZSetKey  = "review:v1:rank:goods:score"
	goodsScoreRankMetaKey  = "review:v1:rank:goods:score:meta"
	cacheDoubleDeleteDelay = 300 * time.Millisecond
	cacheDeleteTimeout     = 3 * time.Second
	// localCacheBackfillTTL is the TTL for local cache backfill after Redis hit.
	localCacheBackfillTTL = 2 * time.Minute
)

func NewReviewRepo(data *Data, logger log.Logger) biz.ReviewRepo {
	return &ReviewRepo{
		data:  data,
		log:   log.NewHelper(logger),
		local: localcache.New(),
	}
}

func (r *ReviewRepo) SaveReview(ctx context.Context, review *model.ReviewInfo) (*model.ReviewInfo, error) {
	if err := r.data.q.ReviewInfo.WithContext(ctx).Create(review); err != nil {
		return nil, wrapReviewDB("create review", err)
	}
	if err := r.invalidateStoreReviewCache(ctx, review.StoreID); err != nil {
		r.log.WithContext(ctx).Warnf("[data] 闁告帗绋戠紓鎾舵嫚閸曨亞骞嗛柛姘缁斿鎮堕崱妤冩殫闂佹崘娅ｇ槐锔锯偓娑櫭妵鎴犳嫻?store_id=%d: %v", review.StoreID, err)
	}
	if err := r.invalidatePublicReviewCache(ctx, review); err != nil {
		r.log.WithContext(ctx).Warnf("[data] invalidate public review cache after create failed review_id=%d: %v", review.ReviewID, err)
	}
	if err := r.invalidateGoodsScoreRankCache(ctx); err != nil {
		r.log.WithContext(ctx).Warnf("[data] 闁告帗绋戠紓鎾舵嫚閸曨亞骞嗛柛姘缁斿鎮堕崱妤佹珜闁告繀娴囬惁搴ㄥ礆閸℃﹩娓风紓鍌涙尭閻°劍寰勬潏顐バ? %v", err)
	}
	return review, nil
}

func (r *ReviewRepo) GetReviewByOrderID(ctx context.Context, orderID int64) ([]*model.ReviewInfo, error) {
	reviews, err := r.data.q.ReviewInfo.WithContext(ctx).Where(r.data.q.ReviewInfo.OrderID.Eq(orderID)).Find()
	if err != nil {
		return nil, wrapReviewDB("query reviews by order", err)
	}
	return reviews, nil
}

func (r *ReviewRepo) GetByReviewID(ctx context.Context, reviewID int64) (*model.ReviewInfo, error) {
	row, err := r.data.q.ReviewInfo.WithContext(ctx).Where(r.data.q.ReviewInfo.ReviewID.Eq(reviewID)).First()
	if err != nil {
		return nil, wrapReviewDB("get review", err)
	}
	return row, nil
}

func (r *ReviewRepo) ListByOrderID(ctx context.Context, p *biz.ReviewListOrderParams) ([]*model.ReviewInfo, int64, error) {
	q := r.data.q.ReviewInfo.WithContext(ctx).Where(r.data.q.ReviewInfo.OrderID.Eq(p.OrderID))
	offset := int((p.Page - 1) * p.PageSize)
	list, total, err := q.Order(r.data.q.ReviewInfo.CreateAt.Desc()).FindByPage(offset, int(p.PageSize))
	if err != nil {
		return nil, 0, wrapReviewDB("list reviews by order", err)
	}
	return list, total, nil
}

func (r *ReviewRepo) ListPending(ctx context.Context, p *biz.ReviewListPendingParams) ([]*model.ReviewInfo, int64, error) {
	q := r.data.q.ReviewInfo.WithContext(ctx).Where(r.data.q.ReviewInfo.Status.Eq(biz.ReviewStatusPending))
	offset := int((p.Page - 1) * p.PageSize)
	list, total, err := q.Order(r.data.q.ReviewInfo.CreateAt.Desc()).FindByPage(offset, int(p.PageSize))
	if err != nil {
		return nil, 0, wrapReviewDB("list pending reviews", err)
	}
	return list, total, nil
}

func (r *ReviewRepo) ListPendingAppeals(ctx context.Context, p *biz.AppealListPendingParams) ([]*model.ReviewAppealInfo, int64, error) {
	q := r.data.q.ReviewAppealInfo.WithContext(ctx).Where(r.data.q.ReviewAppealInfo.Status.Eq(biz.ReviewStatusPending))
	offset := int((p.Page - 1) * p.PageSize)
	list, total, err := q.Order(r.data.q.ReviewAppealInfo.CreateAt.Desc()).FindByPage(offset, int(p.PageSize))
	if err != nil {
		return nil, 0, wrapReviewDB("list pending appeals", err)
	}
	return list, total, nil
}

func (r *ReviewRepo) ListGoodsScoreRank(ctx context.Context, p *biz.GoodsScoreRankParams) ([]*biz.GoodsScoreRankItem, int64, error) {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize < 1 {
		p.PageSize = 10
	}
	offset := int64((p.Page - 1) * p.PageSize)
	limit := int64(p.PageSize)

	if r.data.rdb == nil {
		return r.listGoodsScoreRankFromDB(ctx, offset, limit)
	}
	if err := r.ensureGoodsScoreRankCache(ctx); err != nil {
		r.log.WithContext(ctx).Warnf("[data] ensureGoodsScoreRankCache failed, fallback DB: %v", err)
		return r.listGoodsScoreRankFromDB(ctx, offset, limit)
	}

	total, err := r.data.rdb.ZCard(ctx, goodsScoreRankZSetKey).Result()
	if err != nil {
		return nil, 0, fmt.Errorf("闁哄被鍎撮妤冩嫚閸曨偄鐎婚柟鐑樺笩椤㈡垿骞€缂佹ɑ娈? %w", err)
	}
	if total == 0 {
		return []*biz.GoodsScoreRankItem{}, 0, nil
	}
	end := offset + limit - 1
	zs, err := r.data.rdb.ZRevRangeWithScores(ctx, goodsScoreRankZSetKey, offset, end).Result()
	if err != nil {
		return nil, 0, fmt.Errorf("闁哄被鍎撮妤冩嫚閸曨偄鐎婚柟鐑樺笩椤㈡垿宕氬Δ鍕┾偓? %w", err)
	}
	if len(zs) == 0 {
		return []*biz.GoodsScoreRankItem{}, total, nil
	}
	fields := make([]string, 0, len(zs))
	spuIDs := make([]int64, 0, len(zs))
	for _, z := range zs {
		spuID, convErr := strconv.ParseInt(fmt.Sprint(z.Member), 10, 64)
		if convErr != nil || spuID <= 0 {
			continue
		}
		spuIDs = append(spuIDs, spuID)
		fields = append(fields, fmt.Sprintf("spu:%d:cnt", spuID))
	}
	countVals, err := r.data.rdb.HMGet(ctx, goodsScoreRankMetaKey, fields...).Result()
	if err != nil {
		return nil, 0, fmt.Errorf("闁哄被鍎撮妤冩嫚閸曨偄鐎婚柟鐑樺笩椤㈡垿宕楅崘鈺傛闁? %w", err)
	}
	countBySpu := make(map[int64]int64, len(spuIDs))
	for i := range countVals {
		if i >= len(spuIDs) {
			break
		}
		if countVals[i] == nil {
			continue
		}
		cnt, convErr := strconv.ParseInt(fmt.Sprint(countVals[i]), 10, 64)
		if convErr != nil {
			continue
		}
		countBySpu[spuIDs[i]] = cnt
	}
	list := make([]*biz.GoodsScoreRankItem, 0, len(zs))
	for _, z := range zs {
		spuID, convErr := strconv.ParseInt(fmt.Sprint(z.Member), 10, 64)
		if convErr != nil || spuID <= 0 {
			continue
		}
		list = append(list, &biz.GoodsScoreRankItem{
			SpuID:       spuID,
			AvgScore:    z.Score,
			ReviewCount: countBySpu[spuID],
		})
	}
	return list, total, nil
}

// listByStoreIdFromDB queries store reviews from MySQL.
func (r *ReviewRepo) listByStoreIdFromDB(ctx context.Context, p *biz.ReviewListStoreParams) ([]*model.ReviewInfo, int64, error) {
	q := r.data.q.ReviewInfo.WithContext(ctx).Where(r.data.q.ReviewInfo.StoreID.Eq(p.StoreID))
	offset := int((p.Page - 1) * p.PageSize)
	list, total, err := q.Order(r.data.q.ReviewInfo.CreateAt.Desc()).FindByPage(offset, int(p.PageSize))
	if err != nil {
		return nil, 0, wrapReviewDB("list store reviews", err)
	}
	return list, total, nil
}

func (r *ReviewRepo) listPublicByStoreIdFromDB(ctx context.Context, p *biz.PublicReviewListParams) ([]*model.ReviewInfo, int64, error) {
	q := r.data.q.ReviewInfo.WithContext(ctx).Where(
		r.data.q.ReviewInfo.StoreID.Eq(p.StoreID),
		r.data.q.ReviewInfo.Status.Eq(biz.ReviewStatusApproved),
	)
	offset := int((p.Page - 1) * p.PageSize)
	list, total, err := q.Order(r.data.q.ReviewInfo.CreateAt.Desc()).FindByPage(offset, int(p.PageSize))
	if err != nil {
		return nil, 0, wrapReviewDB("query public store reviews", err)
	}
	return list, total, nil
}

func (r *ReviewRepo) listPublicBySpuIdFromDB(ctx context.Context, p *biz.PublicReviewListParams) ([]*model.ReviewInfo, int64, error) {
	q := r.data.q.ReviewInfo.WithContext(ctx).Where(
		r.data.q.ReviewInfo.SpuID.Eq(p.SpuID),
		r.data.q.ReviewInfo.Status.Eq(biz.ReviewStatusApproved),
	)
	offset := int((p.Page - 1) * p.PageSize)
	list, total, err := q.Order(r.data.q.ReviewInfo.CreateAt.Desc()).FindByPage(offset, int(p.PageSize))
	if err != nil {
		return nil, 0, wrapReviewDB("query public goods reviews", err)
	}
	return list, total, nil
}

func (r *ReviewRepo) UpdateReview(ctx context.Context, row *model.ReviewInfo) error {
	if err := r.data.q.ReviewInfo.WithContext(ctx).Save(row); err != nil {
		return wrapReviewDB("update review", err)
	}
	if err := r.invalidateStoreReviewCache(ctx, row.StoreID); err != nil {
		r.log.WithContext(ctx).Warnf("[data] 闁哄洤鐡ㄩ弻濠勬嫚閸曨亞骞嗛柛姘缁斿鎮堕崱妤冩殫闂佹崘娅ｇ槐锔锯偓娑櫭妵鎴犳嫻?store_id=%d: %v", row.StoreID, err)
	}
	if err := r.invalidatePublicReviewCache(ctx, row); err != nil {
		r.log.WithContext(ctx).Warnf("[data] invalidate public review cache after update failed review_id=%d: %v", row.ReviewID, err)
	}
	if err := r.invalidateGoodsScoreRankCache(ctx); err != nil {
		r.log.WithContext(ctx).Warnf("[data] 闁哄洤鐡ㄩ弻濠勬嫚閸曨亞骞嗛柛姘缁斿鎮堕崱妤佹珜闁告繀娴囬惁搴ㄥ礆閸℃﹩娓风紓鍌涙尭閻°劍寰勬潏顐バ? %v", err)
	}
	return nil
}

func (r *ReviewRepo) DeleteByReviewID(ctx context.Context, reviewID int64) error {
	info, err := r.data.q.ReviewInfo.WithContext(ctx).Where(r.data.q.ReviewInfo.ReviewID.Eq(reviewID)).First()
	if err != nil {
		return wrapReviewDB("query review before delete", err)
	}
	if _, err := r.data.q.ReviewInfo.WithContext(ctx).Delete(info); err != nil {
		return wrapReviewDB("闁告帞濞€濞呭海鎷犻崟顏嗗箚", err)
	}
	if err := r.invalidateStoreReviewCache(ctx, info.StoreID); err != nil {
		r.log.WithContext(ctx).Warnf("[data] 闁告帞濞€濞呭海鎷犻崟顏嗗箚闁告艾瀛╃粩濠氭偠閸℃鏆楅梺鎹愭缁憋妇鈧稒锚閵囨垹鎷?store_id=%d: %v", info.StoreID, err)
	}
	if err := r.invalidatePublicReviewCache(ctx, info); err != nil {
		r.log.WithContext(ctx).Warnf("[data] invalidate public review cache after delete failed review_id=%d: %v", info.ReviewID, err)
	}
	if err := r.invalidateGoodsScoreRankCache(ctx); err != nil {
		r.log.WithContext(ctx).Warnf("[data] 闁告帞濞€濞呭海鎷犻崟顏嗗箚闁告艾瀛╃粩濠氭偠閸℃娅岄柛婵呮祰閻﹀酣宕氶崱姗嗘阜缂傚倹鎸搁悺銊﹀緞鏉堫偉袝: %v", err)
	}
	return nil
}
func (r *ReviewRepo) SaveReply(ctx context.Context, reply *model.ReviewReplyInfo) (*model.ReviewReplyInfo, error) {
	review, err := r.data.q.ReviewInfo.WithContext(ctx).Where(r.data.q.ReviewInfo.ReviewID.Eq(reply.ReviewID)).First()
	if err != nil {
		return nil, errors.New("review not found")
	}
	if review.HasReply == 1 {
		return nil, errors.New("review already replied")
	}
	if review.StoreID != reply.StoreID {
		return nil, errors.New("permission denied to reply review")
	}
	// Save reply and mark review as replied in one transaction.
	review.HasReply = 1
	if err := r.data.q.Transaction(func(tx *query.Query) error {
		if err := tx.ReviewReplyInfo.WithContext(ctx).Save(reply); err != nil {
			return err
		}
		if err := tx.ReviewInfo.WithContext(ctx).Where(tx.ReviewInfo.ReviewID.Eq(review.ReviewID)).Save(review); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, wrapReviewDB("save reply", err)
	}
	if err := r.invalidateStoreReviewCache(ctx, review.StoreID); err != nil {
		r.log.WithContext(ctx).Warnf("[data] invalidate store review cache after reply failed store_id=%d: %v", review.StoreID, err)
	}
	if err := r.invalidatePublicReviewCache(ctx, review); err != nil {
		r.log.WithContext(ctx).Warnf("[data] invalidate public review cache after reply failed review_id=%d: %v", review.ReviewID, err)
	}
	return reply, nil
}

func (r *ReviewRepo) GetByAppealID(ctx context.Context, appealID int64) (*model.ReviewAppealInfo, error) {
	row, err := r.data.q.ReviewAppealInfo.WithContext(ctx).Where(r.data.q.ReviewAppealInfo.AppealID.Eq(appealID)).First()
	if err != nil {
		return nil, wrapReviewDB("get appeal", err)
	}
	return row, nil
}

func (r *ReviewRepo) ListByUseId(ctx context.Context, p *biz.ReviewListUserParams) ([]*model.ReviewInfo, int64, error) {
	q := r.data.q.ReviewInfo.WithContext(ctx).Where(r.data.q.ReviewInfo.UserID.Eq(p.UserID))
	offset := int((p.Page - 1) * p.PageSize)
	list, total, err := q.Order(r.data.q.ReviewInfo.CreateAt.Desc()).FindByPage(offset, int(p.PageSize))
	if err != nil {
		return nil, 0, wrapReviewDB("list reviews by user", err)
	}
	return list, total, nil
}

// SaveAppeal creates or updates a pending appeal.
func (r *ReviewRepo) SaveAppeal(ctx context.Context, appeal *model.ReviewAppealInfo) (*model.ReviewAppealInfo, error) {
	row, err := r.data.q.ReviewAppealInfo.WithContext(ctx).
		Where(r.data.q.ReviewAppealInfo.ReviewID.Eq(appeal.ReviewID),
			r.data.q.ReviewAppealInfo.StoreID.Eq(appeal.StoreID)).
		First()
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, wrapReviewDB("get appeal", err)
	}
	if row != nil && err == nil {
		if row.Status != biz.ReviewStatusPending {
			return nil, errors.New("appeal already audited")
		}
		appeal.ID = row.ID
		appeal.AppealID = row.AppealID
		appeal.CreateBy = row.CreateBy
		appeal.CreateAt = row.CreateAt
		if err := r.data.q.ReviewAppealInfo.WithContext(ctx).Save(appeal); err != nil {
			return nil, wrapReviewDB("update appeal", err)
		}
		return appeal, nil
	}
	if err := r.data.q.ReviewAppealInfo.WithContext(ctx).Create(appeal); err != nil {
		return nil, wrapReviewDB("create appeal", err)
	}
	return appeal, nil
}

// UpdateAppeal saves appeal audit result.
func (r *ReviewRepo) UpdateAppeal(ctx context.Context, row *model.ReviewAppealInfo) error {
	if err := r.data.q.ReviewAppealInfo.WithContext(ctx).Save(row); err != nil {
		return wrapReviewDB("update appeal", err)
	}
	return nil
}
func (r *ReviewRepo) ListByStoreId(ctx context.Context, p *biz.ReviewListStoreParams) ([]*model.ReviewInfo, int64, error) {
	return r.getData2(ctx, p)
}

func (r *ReviewRepo) ListPublicByStoreId(ctx context.Context, p *biz.PublicReviewListParams) ([]*model.ReviewInfo, int64, error) {
	return r.listPublicReviews(ctx, "store", p.StoreID, p)
}

func (r *ReviewRepo) ListPublicBySpuId(ctx context.Context, p *biz.PublicReviewListParams) ([]*model.ReviewInfo, int64, error) {
	return r.listPublicReviews(ctx, "spu", p.SpuID, p)
}

func (r *ReviewRepo) listPublicReviews(ctx context.Context, dimension string, id int64, p *biz.PublicReviewListParams) ([]*model.ReviewInfo, int64, error) {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize < 1 {
		p.PageSize = 10
	}
	from := int((p.Page - 1) * p.PageSize)
	size := int(p.PageSize)
	key := fmt.Sprintf("review:v1:public:%s:%d:offset:%d:limit:%d", dimension, id, from, size)
	if r.data.rdb == nil {
		return r.publicReviewsFromES(ctx, dimension, id, p)
	}
	raw, err := r.getDataBySingleflightForPublic(ctx, key, dimension, id, p)
	if err != nil {
		return nil, 0, err
	}
	var c storeReviewListCache
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, 0, fmt.Errorf("decode public review cache: %w", err)
	}
	if c.Total == 0 && len(c.Reviews) == 0 && !c.EmptyDBChecked {
		dbList, dbTotal, err := r.listPublicReviewsFromDB(ctx, dimension, p)
		if err != nil {
			return nil, 0, err
		}
		if dbTotal > 0 {
			b, mErr := json.Marshal(&storeReviewListCache{Total: dbTotal, Reviews: dbList})
			if mErr == nil {
				if err := r.setCache(ctx, key, b, 10*time.Minute); err != nil {
					r.log.WithContext(ctx).Warnf("[data] backfill public review cache: %v", err)
				}
			}
			return dbList, dbTotal, nil
		}
	}
	return c.Reviews, c.Total, nil
}

func (r *ReviewRepo) listPublicReviewsFromDB(ctx context.Context, dimension string, p *biz.PublicReviewListParams) ([]*model.ReviewInfo, int64, error) {
	if dimension == "spu" {
		return r.listPublicBySpuIdFromDB(ctx, p)
	}
	return r.listPublicByStoreIdFromDB(ctx, p)
}

func (r *ReviewRepo) getData2(ctx context.Context, p *biz.ReviewListStoreParams) ([]*model.ReviewInfo, int64, error) {
	if r.data.rdb == nil {
		return r.storeReviewsFromES(ctx, p)
	}
	from := int((p.Page - 1) * p.PageSize)
	size := int(p.PageSize)
	// v2: JSON storeReviewListCache; old keys may hold incompatible payloads from previous implementations
	key := fmt.Sprintf("review:v2:store_id:%d:offset:%d:limit:%d", p.StoreID, from, size)
	raw, err := r.getDataBySingleflight(ctx, key, p)
	if err != nil {
		return nil, 0, err
	}
	var c storeReviewListCache
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, 0, fmt.Errorf("閹煎瓨顨婇幗鐢垫嫚閸曨亞骞嗙紓鍌涙尭閻°劎鎲撮敐鍥╁灣: %w", err)
	}
	// restored comment
	if c.Total == 0 && len(c.Reviews) == 0 && !c.EmptyDBChecked {
		dbList, dbTotal, err := r.listByStoreIdFromDB(ctx, p)
		if err != nil {
			return nil, 0, err
		}
		if dbTotal > 0 {
			r.log.WithContext(ctx).Infof("[data] ListByStoreId: 缂傚倹鎸搁悺銊︾▔閾忓厜鏁勫ù?MySQL 闁?%d 闁哄妲勭槐婵嗩啅閹绘帗绀€闂侇偀鍋撻柡浣哄瀹撲焦鎯?store_id=%d", dbTotal, p.StoreID)
			b, mErr := json.Marshal(&storeReviewListCache{Total: dbTotal, Reviews: dbList})
			if mErr == nil {
				if err := r.setCache(ctx, key, b, 10*time.Minute); err != nil {
					r.log.WithContext(ctx).Warnf("[data] 闁搞儳鍋涢敐鐐存償濡ゅ懏鎳欓悹鍥у閻滎垳绱撻幘宕囨憼: %v", err)
				}
			}
			return dbList, dbTotal, nil
		}
	}
	return c.Reviews, c.Total, nil
}

// storeReviewsFromES 闁烩晛鐡ㄩ悡?ES闁挎稑鐗呯粭宀€绱撻幘宕囨憼濞戞搩鍘虹换姘扁偓娑欘焽濞堟垹绱掗幘瀵糕偓顖涚▔閳ь剟鎳涙潏鍓х
func (r *ReviewRepo) storeReviewsFromES(ctx context.Context, p *biz.ReviewListStoreParams) ([]*model.ReviewInfo, int64, error) {
	if r.data.es == nil {
		r.log.WithContext(ctx).Infof("[data] ListByStoreId: elasticsearch 闁哄牜浜滈崹鍨叏鐎ｎ亜顕ч柨娑樿嫰濞叉牠鏌呴埀?MySQL store_id=%d", p.StoreID)
		return r.listByStoreIdFromDB(ctx, p)
	}
	from := int((p.Page - 1) * p.PageSize)
	size := int(p.PageSize)
	desc := sortorder.Desc
	// sort by create_at date field
	sortOpts := types.SortOptions{
		SortOptions: map[string]types.FieldSort{
			"create_at": {Order: &desc},
		},
	}
	resp, err := r.data.es.Search().
		Index("review").
		From(from).
		Size(size).
		TrackTotalHits(true).
		Sort(sortOpts).
		Query(&types.Query{
			Bool: &types.BoolQuery{
				Filter: []types.Query{
					{
						Bool: &types.BoolQuery{
							Should: []types.Query{
								{Term: map[string]types.TermQuery{
									"store_id": {Value: p.StoreID},
								}},
								{Term: map[string]types.TermQuery{
									"store_id.keyword": {Value: strconv.FormatInt(p.StoreID, 10)},
								}},
							},
							MinimumShouldMatch: 1,
						},
					},
				},
			},
		}).
		Do(ctx)
	if err != nil {
		r.log.WithContext(ctx).Errorf("[data] ListByStoreId ES 闁哄被鍎撮妤佸緞鏉堫偉袝 store_id=%d from=%d size=%d: %v", p.StoreID, from, size, err)
		return nil, 0, fmt.Errorf("闁圭顦花鐢告煣閻戞鍙€閻犲洢鍨奸惁搴㈢?ES): %w", err)
	}
	var total int64
	if resp.Hits.Total != nil {
		total = resp.Hits.Total.Value
	}
	out := make([]*model.ReviewInfo, 0, len(resp.Hits.Hits))
	for i := range resp.Hits.Hits {
		row, err := reviewInfoFromESSource(resp.Hits.Hits[i].Source_)
		if err != nil {
			r.log.WithContext(ctx).Warnf("閻犲搫鐤囩换鍐籍閻樺磭銆婇悷娆欑稻閻庝粙鎯?ES 闁哄倸娲﹂妴? %v", err)
			continue
		}
		out = append(out, row)
	}
	r.log.WithContext(ctx).Debugf("[data] ListByStoreId store_id=%d es_total=%d parsed=%d", p.StoreID, total, len(out))
	// restored comment
	if total == 0 {
		dbList, dbTotal, err := r.listByStoreIdFromDB(ctx, p)
		if err != nil {
			return nil, 0, err
		}
		if dbTotal > 0 {
			r.log.WithContext(ctx).Infof("[data] ListByStoreId: ES 闁哄啰濮撮幊鈩冪▔椤撴繄绋?MySQL 闁?%d 闁哄妲勭槐婵嗩啅閹绘帗绀€闂侇偀鍋撻柡浣哄瀹撲焦鎯?store_id=%d", dbTotal, p.StoreID)
			return dbList, dbTotal, nil
		}
	}
	return out, total, nil
}

func (r *ReviewRepo) publicReviewsFromES(ctx context.Context, dimension string, id int64, p *biz.PublicReviewListParams) ([]*model.ReviewInfo, int64, error) {
	if r.data.es == nil {
		return r.listPublicReviewsFromDB(ctx, dimension, p)
	}
	from := int((p.Page - 1) * p.PageSize)
	size := int(p.PageSize)
	desc := sortorder.Desc
	field := "store_id"
	if dimension == "spu" {
		field = "spu_id"
	}
	sortOpts := types.SortOptions{
		SortOptions: map[string]types.FieldSort{
			"create_at": {Order: &desc},
		},
	}
	resp, err := r.data.es.Search().
		Index("review").
		From(from).
		Size(size).
		TrackTotalHits(true).
		Sort(sortOpts).
		Query(&types.Query{
			Bool: &types.BoolQuery{
				Filter: []types.Query{
					{
						Bool: &types.BoolQuery{
							Should: []types.Query{
								{Term: map[string]types.TermQuery{
									field: {Value: id},
								}},
								{Term: map[string]types.TermQuery{
									field + ".keyword": {Value: strconv.FormatInt(id, 10)},
								}},
							},
							MinimumShouldMatch: 1,
						},
					},
					{
						Bool: &types.BoolQuery{
							Should: []types.Query{
								{Term: map[string]types.TermQuery{
									"status": {Value: biz.ReviewStatusApproved},
								}},
								{Term: map[string]types.TermQuery{
									"status.keyword": {Value: strconv.FormatInt(int64(biz.ReviewStatusApproved), 10)},
								}},
							},
							MinimumShouldMatch: 1,
						},
					},
				},
			},
		}).
		Do(ctx)
	if err != nil {
		r.log.WithContext(ctx).Warnf("[data] public reviews ES failed dimension=%s id=%d: %v", dimension, id, err)
		return r.listPublicReviewsFromDB(ctx, dimension, p)
	}
	var total int64
	if resp.Hits.Total != nil {
		total = resp.Hits.Total.Value
	}
	out := make([]*model.ReviewInfo, 0, len(resp.Hits.Hits))
	for i := range resp.Hits.Hits {
		row, err := reviewInfoFromESSource(resp.Hits.Hits[i].Source_)
		if err != nil {
			r.log.WithContext(ctx).Warnf("skip public review ES document: %v", err)
			continue
		}
		if row.Status == biz.ReviewStatusApproved {
			out = append(out, row)
		}
	}
	if total == 0 {
		return r.listPublicReviewsFromDB(ctx, dimension, p)
	}
	return out, total, nil
}

func reviewInfoFromESSource(src json.RawMessage) (*model.ReviewInfo, error) {
	if len(src) == 0 {
		return nil, fmt.Errorf("empty _source")
	}
	dec := json.NewDecoder(bytes.NewReader(src))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	return &model.ReviewInfo{
		ID:             esInt64(m, "id"),
		CreateBy:       esString(m, "create_by"),
		UpdateBy:       esString(m, "update_by"),
		CreateAt:       esTime(m, "create_at"),
		UpdateAt:       esTime(m, "update_at"),
		DeleteAt:       esTimePtr(m, "delete_at"),
		Version:        int32(esInt64(m, "version")),
		ReviewID:       esInt64(m, "review_id"),
		Content:        esString(m, "content"),
		Score:          int32(esInt64(m, "score")),
		ServiceScore:   int32(esInt64(m, "service_score")),
		ExpressScore:   int32(esInt64(m, "express_score")),
		HasMedia:       int32(esInt64(m, "has_media")),
		OrderID:        esInt64(m, "order_id"),
		SkuID:          esInt64(m, "sku_id"),
		SpuID:          esInt64(m, "spu_id"),
		StoreID:        esInt64(m, "store_id"),
		UserID:         esInt64(m, "user_id"),
		Anonymous:      int32(esInt64(m, "anonymous")),
		Tags:           esString(m, "tags"),
		PicInfo:        esString(m, "pic_info"),
		VideoInfo:      esString(m, "video_info"),
		Status:         int32(esInt64(m, "status")),
		IsDefault:      int32(esInt64(m, "is_default")),
		HasReply:       int32(esInt64(m, "has_reply")),
		OpReason:       esString(m, "op_reason"),
		OpRemarks:      esString(m, "op_remarks"),
		OpUser:         esString(m, "op_user"),
		GoodsSnapshoot: esString(m, "goods_snapshoot"),
		ExtJSON:        esString(m, "ext_json"),
		CtrlJSON:       esString(m, "ctrl_json"),
	}, nil
}

func esString(m map[string]any, k string) string {
	v, ok := m[k]
	if !ok || v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case json.Number:
		return x.String()
	case float64:
		return strconv.FormatInt(int64(x), 10)
	default:
		return fmt.Sprint(v)
	}
}

func esInt64(m map[string]any, k string) int64 {
	v, ok := m[k]
	if !ok || v == nil {
		return 0
	}
	switch x := v.(type) {
	case json.Number:
		i, _ := x.Int64()
		return i
	case float64:
		return int64(x)
	case string:
		i, _ := strconv.ParseInt(x, 10, 64)
		return i
	case int64:
		return x
	default:
		return 0
	}
}

func esTime(m map[string]any, k string) time.Time {
	s := esString(m, k)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t
		}
	}
	return time.Time{}
}

func esTimePtr(m map[string]any, k string) *time.Time {
	if _, ok := m[k]; !ok {
		return nil
	}
	t := esTime(m, k)
	if t.IsZero() {
		return nil
	}
	return &t
}

func (r *ReviewRepo) listGoodsScoreRankFromDB(ctx context.Context, offset, limit int64) ([]*biz.GoodsScoreRankItem, int64, error) {
	db := r.data.q.ReviewInfo.WithContext(ctx).UnderlyingDB()
	type rankRow struct {
		SpuID       int64   `gorm:"column:spu_id"`
		AvgScore    float64 `gorm:"column:avg_score"`
		ReviewCount int64   `gorm:"column:review_count"`
	}
	var rows []rankRow
	if err := db.Table(model.TableNameReviewInfo).
		Select("spu_id, AVG(score) AS avg_score, COUNT(1) AS review_count").
		Where("status = ? AND spu_id > 0", biz.ReviewStatusApproved).
		Group("spu_id").
		Order("avg_score DESC, review_count DESC, spu_id ASC").
		Offset(int(offset)).
		Limit(int(limit)).
		Scan(&rows).Error; err != nil {
		return nil, 0, wrapReviewDB("闁哄被鍎撮妤呭疮閸℃鎯傞悹鍥у閸ㄥ酣骞掗幒鏇ㄦ斀", err)
	}
	var total int64
	if err := db.Table(model.TableNameReviewInfo).
		Where("status = ? AND spu_id > 0", biz.ReviewStatusApproved).
		Distinct("spu_id").
		Count(&total).Error; err != nil {
		return nil, 0, wrapReviewDB("缂備胶鍠曢鎼佸疮閸℃鎯傞悹鍥у閸ㄥ酣骞掗幒鏇ㄦ斀", err)
	}
	list := make([]*biz.GoodsScoreRankItem, 0, len(rows))
	for i := range rows {
		list = append(list, &biz.GoodsScoreRankItem{
			SpuID:       rows[i].SpuID,
			AvgScore:    rows[i].AvgScore,
			ReviewCount: rows[i].ReviewCount,
		})
	}
	return list, total, nil
}

func (r *ReviewRepo) ensureGoodsScoreRankCache(ctx context.Context) error {
	if r.data.rdb == nil {
		return nil
	}
	exists, err := r.data.rdb.Exists(ctx, goodsScoreRankZSetKey).Result()
	if err != nil {
		return err
	}
	if exists > 0 {
		return nil
	}
	db := r.data.q.ReviewInfo.WithContext(ctx).UnderlyingDB()
	var rows []goodsScoreRankMeta
	if err := db.Table(model.TableNameReviewInfo).
		Select("spu_id, AVG(score) AS avg_score, COUNT(1) AS review_count").
		Where("status = ? AND spu_id > 0", biz.ReviewStatusApproved).
		Group("spu_id").
		Order("avg_score DESC, review_count DESC, spu_id ASC").
		Scan(&rows).Error; err != nil {
		return wrapReviewDB("rebuild goods score rank cache", err)
	}
	pipe := r.data.rdb.Pipeline()
	pipe.Del(ctx, goodsScoreRankZSetKey, goodsScoreRankMetaKey)
	for i := range rows {
		meta := rows[i]
		pipe.ZAdd(ctx, goodsScoreRankZSetKey, redis.Z{
			Score:  meta.AvgScore,
			Member: strconv.FormatInt(meta.SpuID, 10),
		})
		pipe.HSet(ctx, goodsScoreRankMetaKey, fmt.Sprintf("spu:%d:cnt", meta.SpuID), meta.ReviewCount)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("闁告劖鐟ラ崣鍡涘疮閸℃鎯傞悹鍥у閸ㄥ酣骞掗幒鏇ㄦ斀缂傚倹鎸搁悺? %w", err)
	}
	return nil
}

func (r *ReviewRepo) invalidateGoodsScoreRankCache(ctx context.Context) error {
	if r.data.rdb == nil {
		return nil
	}
	if err := r.deleteGoodsScoreRankCacheOnce(ctx); err != nil {
		return err
	}
	r.scheduleDelayedDelete("goods score rank cache", func(c context.Context) error {
		return r.deleteGoodsScoreRankCacheOnce(c)
	})
	return nil
}

var g singleflight.Group

// getDataBySingleflight queries store review list with cache.
func (r *ReviewRepo) getDataBySingleflight(ctx context.Context, key string, p *biz.ReviewListStoreParams) ([]byte, error) {
	v, err, shared := g.Do(key, func() (any, error) {
		data, err := r.getDataFromCahe(ctx, key)
		if err == nil {
			return data, nil
		}
		if errors.Is(err, redis.Nil) {
			reviews, total, err := r.storeReviewsFromES(ctx, p)
			if err != nil {
				return nil, err
			}
			c := storeReviewListCache{
				Total:          total,
				Reviews:        reviews,
				EmptyDBChecked: total == 0 && len(reviews) == 0,
			}
			raw, err := json.Marshal(&c)
			if err != nil {
				return nil, err
			}
			if err := r.setCache(ctx, key, raw, 10*time.Minute); err != nil {
				r.log.WithContext(ctx).Warnf("[data] setCache: %v", err)
			}
			return raw, nil
		}
		return nil, err
	})
	r.log.WithContext(ctx).Debugf("[data] getDataBySingleflight: key=%s shared=%v err=%v", key, shared, err)
	if err != nil {
		return nil, err
	}
	return v.([]byte), nil
}

// 閻犲洩宕电槐锔锯偓娑欙公缁辩檽1 闁哄牜鍓欏﹢?map 闁?L2 Redis
func (r *ReviewRepo) getDataBySingleflightForPublic(ctx context.Context, key string, dimension string, id int64, p *biz.PublicReviewListParams) ([]byte, error) {
	v, err, shared := g.Do(key, func() (any, error) {
		data, err := r.getDataFromCahe(ctx, key)
		if err == nil {
			return data, nil
		}
		if errors.Is(err, redis.Nil) {
			reviews, total, err := r.publicReviewsFromES(ctx, dimension, id, p)
			if err != nil {
				return nil, err
			}
			c := storeReviewListCache{
				Total:          total,
				Reviews:        reviews,
				EmptyDBChecked: total == 0 && len(reviews) == 0,
			}
			raw, err := json.Marshal(&c)
			if err != nil {
				return nil, err
			}
			if err := r.setCache(ctx, key, raw, 10*time.Minute); err != nil {
				r.log.WithContext(ctx).Warnf("[data] set public review cache: %v", err)
			}
			return raw, nil
		}
		return nil, err
	})
	r.log.WithContext(ctx).Debugf("[data] getDataBySingleflightForPublic: key=%s shared=%v err=%v", key, shared, err)
	if err != nil {
		return nil, err
	}
	return v.([]byte), nil
}

func (r *ReviewRepo) getDataFromCahe(ctx context.Context, key string) (any, error) {
	if r.local != nil {
		if b, ok := r.local.Get(key); ok {
			r.log.WithContext(ctx).Debugf("[data] getDataFromCahe: local hit key=%s", key)
			return b, nil
		}
	}
	if r.data.rdb == nil {
		return nil, redis.Nil
	}
	b, err := r.data.rdb.Get(ctx, key).Bytes()
	if err == nil && r.local != nil {
		r.local.Set(key, b, localCacheBackfillTTL)
		r.log.WithContext(ctx).Debugf("[data] getDataFromCahe: redis hit, backfill local key=%s", key)
	}
	return b, err
}

// 闁告劖鐟х槐锔锯偓娑欙公缁变即宕ョ仦鐐槯闁告劖鐟﹀﹢浼村捶妫颁胶鐟?Redis
func (r *ReviewRepo) setCache(ctx context.Context, key string, data any, ttl time.Duration) error {
	r.log.WithContext(ctx).Debugf("[data] setCache: key=%s ttl=%s", key, ttl)
	if raw, ok := cacheBytes(data); ok && r.local != nil {
		r.local.Set(key, raw, ttl)
	}
	if r.data.rdb == nil {
		return nil
	}
	return r.data.rdb.Set(ctx, key, data, ttl).Err()
}

func cacheBytes(data any) ([]byte, bool) {
	switch v := data.(type) {
	case []byte:
		return v, true
	case string:
		return []byte(v), true
	default:
		return nil, false
	}
}

func (r *ReviewRepo) invalidateStoreReviewCache(ctx context.Context, storeID int64) error {
	if storeID <= 0 {
		return nil
	}
	if r.local != nil {
		r.local.DeletePrefix(fmt.Sprintf("review:v2:store_id:%d:", storeID))
	}
	if r.data.rdb == nil {
		return nil
	}
	if err := r.deleteStoreReviewCacheOnce(ctx, storeID); err != nil {
		return err
	}
	r.scheduleDelayedDelete(fmt.Sprintf("store review cache store_id=%d", storeID), func(c context.Context) error {
		return r.deleteStoreReviewCacheOnce(c, storeID)
	})
	return nil
}

func (r *ReviewRepo) invalidatePublicReviewCache(ctx context.Context, row *model.ReviewInfo) error {
	if row == nil {
		return nil
	}
	if err := r.invalidatePublicReviewCacheByDimension(ctx, "store", row.StoreID); err != nil {
		return err
	}
	if row.SpuID > 0 {
		if err := r.invalidatePublicReviewCacheByDimension(ctx, "spu", row.SpuID); err != nil {
			return err
		}
	}
	return nil
}

func (r *ReviewRepo) invalidatePublicReviewCacheByDimension(ctx context.Context, dimension string, id int64) error {
	if id <= 0 {
		return nil
	}
	prefix := fmt.Sprintf("review:v1:public:%s:%d:", dimension, id)
	if r.local != nil {
		r.local.DeletePrefix(prefix)
	}
	if r.data.rdb == nil {
		return nil
	}
	if err := r.deleteByPrefix(ctx, prefix); err != nil {
		return err
	}
	r.scheduleDelayedDelete(fmt.Sprintf("public review cache %s=%d", dimension, id), func(c context.Context) error {
		return r.deleteByPrefix(c, prefix)
	})
	return nil
}

func (r *ReviewRepo) deleteGoodsScoreRankCacheOnce(ctx context.Context) error {
	if err := r.data.rdb.Del(ctx, goodsScoreRankZSetKey, goodsScoreRankMetaKey).Err(); err != nil {
		return fmt.Errorf("婵炴挸鎳愰幃濠囧疮閸℃鎯傞悹鍥у閸ㄥ酣骞掗幒鏇ㄦ斀缂傚倹鎸搁悺? %w", err)
	}
	return nil
}

func (r *ReviewRepo) deleteStoreReviewCacheOnce(ctx context.Context, storeID int64) error {
	return r.deleteByPrefix(ctx, fmt.Sprintf("review:v2:store_id:%d:", storeID))
}

func (r *ReviewRepo) deleteByPrefix(ctx context.Context, prefix string) error {
	pattern := prefix + "*"
	var cursor uint64
	for {
		keys, nextCursor, err := r.data.rdb.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return fmt.Errorf("scan store cache: %w", err)
		}
		if len(keys) > 0 {
			if err := r.data.rdb.Del(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("delete store cache: %w", err)
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return nil
}

func (r *ReviewRepo) scheduleDelayedDelete(scene string, deleteFn func(context.Context) error) {
	go func() {
		time.Sleep(cacheDoubleDeleteDelay)
		ctx, cancel := context.WithTimeout(context.Background(), cacheDeleteTimeout)
		defer cancel()
		if err := deleteFn(ctx); err != nil {
			r.log.WithContext(ctx).Warnf("[data] 鐎点倖鍎肩换婊堝矗鐏炶棄鐏╁鎯扮簿鐟?scene=%s err=%v", scene, err)
		}
	}()
}

// getDataBySingleflight queries and caches store review list.

// SaveAppeal creates or updates a pending appeal.
