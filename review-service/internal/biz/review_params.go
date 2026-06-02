package biz

// UpdateReviewParams 用户修改评价
type UpdateReviewParams struct {
	ReviewID       int64
	OperatorUserID int64
	Score          int32
	ServiceScore   int32
	ExpressScore   int32
	Content        string
	PicInfo        string
	VideoInfo      string
}

// DeleteReviewParams 用户删除评价
type DeleteReviewParams struct {
	ReviewID       int64
	OperatorUserID int64
}

// ReviewListOrderParams 按订单分页列表
type ReviewListOrderParams struct {
	OrderID  int64
	Page     int32
	PageSize int32
}

// ReviewListUserParams 按用户分页列表
type ReviewListUserParams struct {
	UserID   int64
	Page     int32
	PageSize int32
}

// ReviewListStoreParams 按店铺分页列表
type ReviewListStoreParams struct {
	StoreID  int64
	Page     int32
	PageSize int32
}

// PublicReviewListParams is for consumer-facing review feeds.
type PublicReviewListParams struct {
	StoreID  int64
	SpuID    int64
	Page     int32
	PageSize int32
}

// ReviewListPendingParams 运营端待审核分页列表
type ReviewListPendingParams struct {
	Page     int32
	PageSize int32
}

// AppealListPendingParams 运营端待审核申诉分页列表
type AppealListPendingParams struct {
	Page     int32
	PageSize int32
}

// GoodsScoreRankParams 商品评分排行分页参数
type GoodsScoreRankParams struct {
	Page     int32
	PageSize int32
}

// GoodsScoreRankItem 商品评分排行项
type GoodsScoreRankItem struct {
	SpuID       int64
	AvgScore    float64
	ReviewCount int64
}

// AuditReviewParams 运营审核评价
type AuditReviewParams struct {
	ReviewID int64
	Result   int32
	Remark   string
	Operator string
}

// AppealReviewParams 商家发起申诉
type AppealReviewParams struct {
	UserID   int64
	StoreID  int64
	ReviewID int64
	Reason   string
	PicInfo  string
}

// AuditAppealParams 运营审核申诉
type AuditAppealParams struct {
	AppealID int64
	StoreID  int64
	Result   int32
	Remark   string
	Operator string
}
