package service

import (
	"context"

	pb "review-service/api/review/v1"
	"review-service/internal/biz"
	"review-service/internal/data/model"
)

type GoodsScoreRankItem struct {
	SpuID       int64   `json:"spuID"`
	AvgScore    float64 `json:"avgScore"`
	ReviewCount int64   `json:"reviewCount"`
}

type ReviewService struct {
	pb.UnimplementedReviewServer
	uc *biz.ReviewUsecase
}

func NewReviewService(uc *biz.ReviewUsecase) *ReviewService {
	return &ReviewService{
		uc: uc,
	}
}

func reviewInfoToListItem(m *model.ReviewInfo) *pb.ReviewListItem {
	if m == nil {
		return nil
	}
	return &pb.ReviewListItem{
		ReviewID:     m.ReviewID,
		UserID:       m.UserID,
		OrderID:      m.OrderID,
		Score:        m.Score,
		ServiceScore: m.ServiceScore,
		ExpressScore: m.ExpressScore,
		Content:      m.Content,
		PicInfo:      m.PicInfo,
		VideoInfo:    m.VideoInfo,
		Status:       m.Status,
		HasReply:     m.HasReply,
		CreateAt:     m.CreateAt.UnixMilli(),
	}
}

func appealInfoToListItem(m *model.ReviewAppealInfo) *pb.AppealListItem {
	if m == nil {
		return nil
	}
	return &pb.AppealListItem{
		AppealID:  m.AppealID,
		ReviewID:  m.ReviewID,
		StoreID:   m.StoreID,
		Status:    m.Status,
		Reason:    m.Reason,
		Content:   m.Content,
		PicInfo:   m.PicInfo,
		VideoInfo: m.VideoInfo,
		OpRemarks: m.OpRemarks,
		OpUser:    m.OpUser,
		CreateAt:  m.CreateAt.UnixMilli(),
	}
}

func (s *ReviewService) CreateReview(ctx context.Context, req *pb.CreateReviewRequest) (*pb.CreateReviewReply, error) {
	if err := req.Validate(); err != nil {
		return nil, pb.ErrorInvalidParameter("%v", err)
	}
	anonymous := int32(0)
	if req.Anonymous {
		anonymous = 1
	}
	review, err := s.uc.CreateReview(ctx, &model.ReviewInfo{
		UserID:       req.UserID,
		OrderID:      req.OrderID,
		StoreID:      req.StoreID,
		Score:        req.Score,
		ServiceScore: req.ServiceScore,
		ExpressScore: req.ExpressScore,
		Content:      req.Content,
		PicInfo:      req.PicInfo,
		VideoInfo:    req.VideoInfo,
		Status:       biz.ReviewStatusPending,
		Anonymous:    anonymous,
	})
	if err != nil {
		return nil, err
	}
	return &pb.CreateReviewReply{ReviewID: review.ReviewID}, nil
}

func (s *ReviewService) UpdateReview(ctx context.Context, req *pb.UpdateReviewRequest) (*pb.UpdateReviewReply, error) {
	if err := req.Validate(); err != nil {
		return nil, pb.ErrorInvalidParameter("%v", err)
	}
	if _, err := s.uc.UpdateReview(ctx, &biz.UpdateReviewParams{
		ReviewID:       req.ReviewID,
		OperatorUserID: req.UserID,
		Score:          req.Score,
		ServiceScore:   req.ServiceScore,
		ExpressScore:   req.ExpressScore,
		Content:        req.Content,
		PicInfo:        req.PicInfo,
		VideoInfo:      req.VideoInfo,
	}); err != nil {
		return nil, err
	}
	return &pb.UpdateReviewReply{}, nil
}

func (s *ReviewService) DeleteReview(ctx context.Context, req *pb.DeleteReviewRequest) (*pb.DeleteReviewReply, error) {
	if err := req.Validate(); err != nil {
		return nil, pb.ErrorInvalidParameter("%v", err)
	}
	err := s.uc.DeleteReview(ctx, &biz.DeleteReviewParams{ReviewID: req.ReviewID, OperatorUserID: req.UserID})
	if err != nil {
		return nil, err
	}
	return &pb.DeleteReviewReply{}, nil
}

func (s *ReviewService) GetReview(ctx context.Context, req *pb.GetReviewRequest) (*pb.GetReviewReply, error) {
	if err := req.Validate(); err != nil {
		return nil, pb.ErrorInvalidParameter("%v", err)
	}
	review, err := s.uc.GetReview(ctx, req.ReviewID)
	if err != nil {
		return nil, err
	}
	return &pb.GetReviewReply{Item: reviewInfoToListItem(review)}, nil
}

func (s *ReviewService) ListReview(ctx context.Context, req *pb.ListReviewRequest) (*pb.ListReviewReply, error) {
	if err := req.Validate(); err != nil {
		return nil, pb.ErrorInvalidParameter("%v", err)
	}
	reviews, total, err := s.uc.ListReview(ctx, &biz.ReviewListOrderParams{
		OrderID:  req.OrderID,
		Page:     req.Page,
		PageSize: req.PageSize,
	})
	if err != nil {
		return nil, err
	}
	list := make([]*pb.ReviewListItem, 0, len(reviews))
	for _, r := range reviews {
		list = append(list, reviewInfoToListItem(r))
	}
	return &pb.ListReviewReply{List: list, Total: total}, nil
}

func (s *ReviewService) ListPendingReviews(ctx context.Context, req *pb.ListPendingReviewsRequest) (*pb.ListReviewReply, error) {
	if err := req.Validate(); err != nil {
		return nil, pb.ErrorInvalidParameter("%v", err)
	}
	reviews, total, err := s.uc.ListPendingReviews(ctx, &biz.ReviewListPendingParams{
		Page:     req.Page,
		PageSize: req.PageSize,
	})
	if err != nil {
		return nil, err
	}
	list := make([]*pb.ReviewListItem, 0, len(reviews))
	for _, r := range reviews {
		list = append(list, reviewInfoToListItem(r))
	}
	return &pb.ListReviewReply{List: list, Total: total}, nil
}

func (s *ReviewService) ListPendingAppeals(ctx context.Context, req *pb.ListPendingAppealsRequest) (*pb.ListPendingAppealsReply, error) {
	if err := req.Validate(); err != nil {
		return nil, pb.ErrorInvalidParameter("%v", err)
	}
	appeals, total, err := s.uc.ListPendingAppeals(ctx, &biz.AppealListPendingParams{
		Page:     req.Page,
		PageSize: req.PageSize,
	})
	if err != nil {
		return nil, err
	}
	list := make([]*pb.AppealListItem, 0, len(appeals))
	for _, a := range appeals {
		list = append(list, appealInfoToListItem(a))
	}
	return &pb.ListPendingAppealsReply{List: list, Total: total}, nil
}

func (s *ReviewService) AuditReview(ctx context.Context, req *pb.AuditReviewRequest) (*pb.AuditReviewReply, error) {
	if err := req.Validate(); err != nil {
		return nil, pb.ErrorInvalidParameter("%v", err)
	}
	review, err := s.uc.AuditReview(ctx, &biz.AuditReviewParams{
		ReviewID: req.ReviewID,
		Result:   req.Result,
		Remark:   req.Remark,
		Operator: req.Operator,
	})
	if err != nil {
		return nil, err
	}
	return &pb.AuditReviewReply{ReviewID: review.ReviewID, Status: review.Status}, nil
}

func (s *ReviewService) ReplyReview(ctx context.Context, req *pb.ReplyReviewRequest) (*pb.ReplyReviewReply, error) {
	if err := req.Validate(); err != nil {
		return nil, pb.ErrorInvalidParameter("%v", err)
	}
	reply, err := s.uc.CreateReply(ctx, &model.ReviewReplyInfo{
		ReviewID:  req.ReviewID,
		StoreID:   req.StoreID,
		Content:   req.Content,
		PicInfo:   req.PicInfo,
		VideoInfo: req.VideoInfo,
		ExtJSON:   req.ExtJSON,
		CtrlJSON:  req.CtrlJSON,
	})
	if err != nil {
		return nil, err
	}
	return &pb.ReplyReviewReply{ReplyID: reply.ReplyID}, nil
}

func (s *ReviewService) AppealReview(ctx context.Context, req *pb.AppealReviewRequest) (*pb.AppealReviewReply, error) {
	if err := req.Validate(); err != nil {
		return nil, pb.ErrorInvalidParameter("%v", err)
	}
	appeal, err := s.uc.AppealReview(ctx, &biz.AppealReviewParams{
		UserID:   req.UserID,
		ReviewID: req.ReviewID,
		Reason:   req.Reason,
		PicInfo:  req.PicInfo,
	})
	if err != nil {
		return nil, err
	}
	return &pb.AppealReviewReply{AppealID: appeal.AppealID}, nil
}

func (s *ReviewService) AuditAppeal(ctx context.Context, req *pb.AuditAppealRequest) (*pb.AuditAppealReply, error) {
	if err := req.Validate(); err != nil {
		return nil, pb.ErrorInvalidParameter("%v", err)
	}
	_, err := s.uc.AuditAppeal(ctx, &biz.AuditAppealParams{
		AppealID: req.AppealID,
		Result:   req.Result,
		Remark:   req.Remark,
		Operator: req.Operator,
	})
	if err != nil {
		return nil, err
	}
	return &pb.AuditAppealReply{}, nil
}

func (s *ReviewService) ListReviewByUseId(ctx context.Context, req *pb.ListReviewByUseIdRequest) (*pb.ListReviewReply, error) {
	if err := req.Validate(); err != nil {
		return nil, pb.ErrorInvalidParameter("%v", err)
	}
	reviews, total, err := s.uc.ListReviewByUseId(ctx, &biz.ReviewListUserParams{
		UserID:   req.UserID,
		Page:     req.Page,
		PageSize: req.PageSize,
	})
	if err != nil {
		return nil, err
	}
	list := make([]*pb.ReviewListItem, 0, len(reviews))
	for _, r := range reviews {
		list = append(list, reviewInfoToListItem(r))
	}
	return &pb.ListReviewReply{List: list, Total: total}, nil
}

func (s *ReviewService) ListReviewByStoreId(ctx context.Context, req *pb.ListReviewByStoreIdRequest) (*pb.ListReviewByStoreIdReply, error) {
	if err := req.Validate(); err != nil {
		return nil, pb.ErrorInvalidParameter("%v", err)
	}
	reviews, total, err := s.uc.ListReviewByStoreId(ctx, &biz.ReviewListStoreParams{
		StoreID:  req.StoreID,
		Page:     req.Page,
		PageSize: req.PageSize,
	})
	if err != nil {
		return nil, err
	}
	list := make([]*pb.ReviewListItem, 0, len(reviews))
	for _, r := range reviews {
		list = append(list, reviewInfoToListItem(r))
	}
	return &pb.ListReviewByStoreIdReply{List: list, Total: total}, nil
}

func (s *ReviewService) ListPublicReviewsByStoreId(ctx context.Context, storeID int64, page, pageSize int32) (*pb.ListReviewReply, error) {
	reviews, total, err := s.uc.ListPublicReviewsByStoreId(ctx, &biz.PublicReviewListParams{
		StoreID:  storeID,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		return nil, err
	}
	list := make([]*pb.ReviewListItem, 0, len(reviews))
	for _, r := range reviews {
		list = append(list, reviewInfoToListItem(r))
	}
	return &pb.ListReviewReply{List: list, Total: total}, nil
}

func (s *ReviewService) ListPublicReviewsBySpuId(ctx context.Context, spuID int64, page, pageSize int32) (*pb.ListReviewReply, error) {
	reviews, total, err := s.uc.ListPublicReviewsBySpuId(ctx, &biz.PublicReviewListParams{
		SpuID:    spuID,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		return nil, err
	}
	list := make([]*pb.ReviewListItem, 0, len(reviews))
	for _, r := range reviews {
		list = append(list, reviewInfoToListItem(r))
	}
	return &pb.ListReviewReply{List: list, Total: total}, nil
}

func (s *ReviewService) ListGoodsScoreRank(ctx context.Context, page, pageSize int32) ([]*GoodsScoreRankItem, int64, error) {
	list, total, err := s.uc.ListGoodsScoreRank(ctx, &biz.GoodsScoreRankParams{
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		return nil, 0, err
	}
	out := make([]*GoodsScoreRankItem, 0, len(list))
	for i := range list {
		out = append(out, &GoodsScoreRankItem{
			SpuID:       list[i].SpuID,
			AvgScore:    list[i].AvgScore,
			ReviewCount: list[i].ReviewCount,
		})
	}
	return out, total, nil
}
