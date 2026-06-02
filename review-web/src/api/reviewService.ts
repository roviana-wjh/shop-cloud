import { request } from './http'
import type {
  AppealReviewPayload,
  CreateReviewPayload,
  DeleteReviewPayload,
  GoodsScoreRankItem,
  ListReply,
  PaginationParams,
  ReviewListItem,
  UploadImageReply,
  UpdateReviewPayload,
} from '../types/review'

export function createReview(payload: CreateReviewPayload) {
  return request<{ reviewID: number }>('review', {
    method: 'POST',
    url: '/v1/review',
    data: payload,
  })
}

export function updateReview(payload: UpdateReviewPayload) {
  return request<void>('review', {
    method: 'PUT',
    url: `/v1/review/${payload.reviewID}`,
    data: payload,
  })
}

export function deleteReview(payload: DeleteReviewPayload) {
  return request<void>('review', {
    method: 'DELETE',
    url: `/v1/review/${payload.reviewID}`,
    data: payload,
  })
}

export function getReview(reviewID: number) {
  return request<{ item: ReviewListItem }>('review', {
    method: 'GET',
    url: `/v1/review/detail/${reviewID}`,
  })
}

export function listReviewByUser(userID: number, params: PaginationParams) {
  return request<ListReply<ReviewListItem>>('review', {
    method: 'GET',
    url: `/v1/review/user/${userID}`,
    params,
  })
}

export function listReviewByOrder(orderID: number, params: PaginationParams) {
  return request<ListReply<ReviewListItem>>('review', {
    method: 'GET',
    url: `/v1/review/order/${orderID}`,
    params,
  })
}

export function listReviewByStore(storeID: number, params: PaginationParams) {
  return request<ListReply<ReviewListItem>>('review', {
    method: 'GET',
    url: `/v1/review/store/${storeID}`,
    params,
  })
}

export function listPublicReviewsByStore(storeID: number, params: PaginationParams) {
  return request<ListReply<ReviewListItem>>('review', {
    method: 'GET',
    url: `/v1/review/public/store/${storeID}`,
    params,
  })
}

export function listPublicReviewsByGoods(spuID: number, params: PaginationParams) {
  return request<ListReply<ReviewListItem>>('review', {
    method: 'GET',
    url: `/v1/review/public/goods/${spuID}`,
    params,
  })
}

export function listGoodsScoreRank(params: PaginationParams) {
  return request<ListReply<GoodsScoreRankItem>>('review', {
    method: 'GET',
    url: '/v1/review/rank/goods/score',
    params,
  })
}

export function appealReview(payload: AppealReviewPayload) {
  return request<{ appealID: number }>('review', {
    method: 'POST',
    url: '/v1/review/appeal',
    data: payload,
  })
}

export function uploadReviewImage(file: FormData) {
  return request<UploadImageReply>('review', {
    method: 'POST',
    url: '/v1/upload/review-image',
    data: file,
  })
}
