<script setup lang="ts">
import { computed, reactive, ref, watchEffect } from 'vue'
import { ElMessage } from 'element-plus'
import AppShell from '../../layouts/AppShell.vue'
import LocalImageUploader from '../../components/LocalImageUploader.vue'
import {
  createReview,
  deleteReview,
  getReview,
  listPublicReviewsByGoods,
  listPublicReviewsByStore,
  listReviewByOrder,
  listReviewByUser,
  updateReview,
} from '../../api/reviewService'
import { useAppStore } from '../../stores/app'
import type { CreateReviewPayload, ReviewListItem, UpdateReviewPayload } from '../../types/review'

const store = useAppStore()

const createForm = reactive<CreateReviewPayload>({
  userID: 10001,
  orderID: 20001,
  storeID: 30001,
  score: 5,
  serviceScore: 5,
  expressScore: 5,
  content: '',
  picInfo: '[]',
  videoInfo: '[]',
  anonymous: false,
})

const editForm = reactive<UpdateReviewPayload>({
  reviewID: 0,
  userID: 10001,
  score: 5,
  serviceScore: 5,
  expressScore: 5,
  content: '',
  picInfo: '[]',
  videoInfo: '[]',
})

const detailReviewID = ref(0)
const userQuery = reactive({ userID: 10001, page: 1, pageSize: 10 })
const orderQuery = reactive({ orderID: 20001, page: 1, pageSize: 10 })
const publicQuery = reactive({ mode: 'store' as 'store' | 'goods', storeID: 30001, spuID: 10001, page: 1, pageSize: 10 })

const userList = ref<ReviewListItem[]>([])
const orderList = ref<ReviewListItem[]>([])
const publicList = ref<ReviewListItem[]>([])
const detail = ref<ReviewListItem | null>(null)
const userTotal = ref(0)
const orderTotal = ref(0)
const publicTotal = ref(0)
const loading = ref(false)

const showcaseProduct = computed(() => ({
  title: detail.value?.content ? '评价中的精选商品' : '夏日轻盈防晒外套',
  subtitle: '百亿补贴风格展示位，图片暂用空白占位',
  salesText: detail.value ? `订单 ${detail.value.orderID}` : '已拼 12.8 万件',
  score: detail.value?.score ?? createForm.score,
}))

const summaryStats = computed(() => [
  { label: '我的评价', value: userTotal.value || userList.value.length, hint: '消费者评价资产' },
  { label: '订单评价', value: orderTotal.value || orderList.value.length, hint: '订单维度追踪' },
  { label: '当前评分', value: `${createForm.score}.0`, hint: '商品满意度感知' },
  { label: '匿名模式', value: createForm.anonymous ? '开启' : '关闭', hint: '晒单身份控制' },
])

const reviewFeed = computed(() => {
  const source = userList.value.length ? userList.value : orderList.value
  if (source.length) {
    return source.slice(0, 6)
  }
  if (detail.value) {
    return [detail.value]
  }
  return []
})

function formatTime(timestamp: number) {
  if (!timestamp) {
    return '刚刚'
  }
  return new Date(timestamp).toLocaleString()
}

watchEffect(() => {
  const session = store.currentSession
  if (!session || session.role !== 'consumer') {
    return
  }
  const userID = Number(session.identity) || 10001
  createForm.userID = userID
  editForm.userID = userID
  userQuery.userID = userID
})

async function handleCreateReview() {
  if (!createForm.storeID || createForm.storeID <= 0) {
    ElMessage.warning('店铺 ID 必须大于 0，请填写与商家端一致的店铺 ID')
    return
  }
  loading.value = true
  try {
    const result = await createReview(createForm)
    ElMessage.success(`评价创建成功，reviewID=${result.reviewID}`)
  } catch (error) {
    ElMessage.error((error as Error).message)
  } finally {
    loading.value = false
  }
}

async function handleLoadUserReviews() {
  loading.value = true
  try {
    const result = await listReviewByUser(userQuery.userID, {
      page: userQuery.page,
      pageSize: userQuery.pageSize,
    })
    userList.value = result.list ?? []
    userTotal.value = result.total ?? 0
  } catch (error) {
    ElMessage.error((error as Error).message)
  } finally {
    loading.value = false
  }
}

async function handleLoadOrderReviews() {
  loading.value = true
  try {
    const result = await listReviewByOrder(orderQuery.orderID, {
      page: orderQuery.page,
      pageSize: orderQuery.pageSize,
    })
    orderList.value = result.list ?? []
    orderTotal.value = result.total ?? 0
  } catch (error) {
    ElMessage.error((error as Error).message)
  } finally {
    loading.value = false
  }
}

async function handleLoadPublicReviews() {
  loading.value = true
  try {
    const params = {
      page: publicQuery.page,
      pageSize: publicQuery.pageSize,
    }
    const result = publicQuery.mode === 'store'
      ? await listPublicReviewsByStore(publicQuery.storeID, params)
      : await listPublicReviewsByGoods(publicQuery.spuID, params)
    publicList.value = result.list ?? []
    publicTotal.value = result.total ?? 0
  } catch (error) {
    ElMessage.error((error as Error).message)
  } finally {
    loading.value = false
  }
}

async function handleLoadDetail() {
  if (!detailReviewID.value) {
    ElMessage.warning('请输入 reviewID')
    return
  }

  loading.value = true
  try {
    const result = await getReview(detailReviewID.value)
    detail.value = result.item
    if (result.item) {
      Object.assign(editForm, {
        reviewID: result.item.reviewID,
        userID: result.item.userID,
        score: result.item.score,
        serviceScore: result.item.serviceScore,
        expressScore: result.item.expressScore,
        content: result.item.content,
        picInfo: result.item.picInfo,
        videoInfo: result.item.videoInfo,
      })
    }
  } catch (error) {
    ElMessage.error((error as Error).message)
  } finally {
    loading.value = false
  }
}

async function handleUpdateReview() {
  loading.value = true
  try {
    await updateReview(editForm)
    ElMessage.success('评价修改成功')
  } catch (error) {
    ElMessage.error((error as Error).message)
  } finally {
    loading.value = false
  }
}

async function handleDeleteReview() {
  if (!editForm.reviewID) {
    ElMessage.warning('请先加载要删除的评价')
    return
  }

  loading.value = true
  try {
    await deleteReview({ reviewID: editForm.reviewID, userID: editForm.userID })
    detail.value = null
    ElMessage.success('评价删除成功')
  } catch (error) {
    ElMessage.error((error as Error).message)
  } finally {
    loading.value = false
  }
}

function statusLabel(status: number) {
  if (status === 10) return '待审核'
  if (status === 20) return '审核通过'
  if (status === 30) return '审核驳回'
  if (status === 40) return '隐藏'
  return `未知(${status})`
}
</script>

<template>
  <AppShell>
    <div class="page-stack">
      <div class="hero-grid">
        <section class="promo-card">
          <div class="shell-badge">限时推荐</div>
          <h2>万人团爆款评价专区</h2>
          <p>模拟拼团电商的商品详情氛围，先把评价、晒单、订单维度能力放进一个页面工作台。</p>
          <div class="promo-tags">
            <span>百亿补贴感</span>
            <span>晒单占位图</span>
            <span>评价可编辑</span>
            <span>订单可追溯</span>
          </div>
        </section>

        <el-card class="commerce-card hero-side-card" shadow="never">
          <div class="section-header">
            <div>
              <h3 class="section-title">商品信息卡</h3>
              <p class="section-subtitle">{{ showcaseProduct.subtitle }}</p>
            </div>
            <el-tag type="danger" round>热销中</el-tag>
          </div>
          <div class="product-panel">
            <div class="placeholder-image">
              <span>商品主图占位</span>
            </div>
            <div class="product-info">
              <h3>{{ showcaseProduct.title }}</h3>
              <div class="price-row">
                <span class="price-main">¥89.00</span>
                <span class="price-origin">¥129.00</span>
                <el-tag type="warning" round>{{ showcaseProduct.salesText }}</el-tag>
              </div>
              <el-rate :model-value="showcaseProduct.score" disabled />
              <div class="feature-list">
                <span>退货包运费</span>
                <span>极速发货</span>
                <span>7 天无理由</span>
                <span>晒单返券</span>
              </div>
              <p class="section-subtitle">图片暂时为空白占位，后续可替换为真实商品图或 CDN 地址。</p>
            </div>
          </div>
        </el-card>
      </div>

      <div class="stats-grid">
        <div v-for="item in summaryStats" :key="item.label" class="stats-card">
          <span class="label">{{ item.label }}</span>
          <span class="value">{{ item.value }}</span>
          <span class="hint">{{ item.hint }}</span>
        </div>
      </div>

      <el-card class="commerce-card table-card" shadow="never">
        <div class="section-header">
          <div>
            <h3 class="section-title">公开评论列表</h3>
            <p class="section-subtitle">面向商品详情页和店铺页，只展示审核通过的评论</p>
          </div>
          <el-tag type="success" round>高流量读取</el-tag>
        </div>
        <div class="table-toolbar">
          <el-radio-group v-model="publicQuery.mode">
            <el-radio-button value="store">店铺</el-radio-button>
            <el-radio-button value="goods">商品</el-radio-button>
          </el-radio-group>
          <el-input-number v-if="publicQuery.mode === 'store'" v-model="publicQuery.storeID" :min="1" />
          <el-input-number v-else v-model="publicQuery.spuID" :min="1" />
          <el-input-number v-model="publicQuery.page" :min="1" />
          <el-input-number v-model="publicQuery.pageSize" :min="1" :max="100" />
          <el-button type="primary" :loading="loading" @click="handleLoadPublicReviews">加载公开评论</el-button>
        </div>
        <div v-if="publicList.length" class="review-card-list">
          <article v-for="row in publicList" :key="row.reviewID" class="review-card">
            <div class="placeholder-image review-thumb">
              <span>晒单图</span>
            </div>
            <div>
              <div class="review-card-header">
                <div>
                  <p class="review-card-title">公开评论 #{{ row.reviewID }}</p>
                  <p class="review-card-meta">{{ formatTime(row.createAt) }}</p>
                </div>
                <el-tag type="success" round>已通过</el-tag>
              </div>
              <el-rate :model-value="row.score" disabled />
              <p class="review-card-content">{{ row.content || '暂无文字内容' }}</p>
              <div class="review-card-tags">
                <span>用户 {{ row.userID }}</span>
                <span>订单 {{ row.orderID }}</span>
                <span>服务 {{ row.serviceScore }}</span>
                <span>物流 {{ row.expressScore }}</span>
              </div>
            </div>
          </article>
        </div>
        <el-empty v-else description="暂无公开评论，审核通过后会出现在这里" />
        <div class="table-footer">共 {{ publicTotal }} 条</div>
      </el-card>

      <div class="panel-grid">
        <el-card class="commerce-card" shadow="never">
          <div class="section-header">
            <div>
              <h3 class="section-title">发表买家秀</h3>
              <p class="section-subtitle">模拟电商详情页下单后评价入口</p>
            </div>
            <el-tag type="success" round>晒单发布</el-tag>
          </div>
          <div class="dense-form">
            <div class="dense-form-grid">
              <div>
                <label class="mini-note">用户 ID</label>
                <el-input-number v-model="createForm.userID" :min="1" />
              </div>
              <div>
                <label class="mini-note">订单 ID</label>
                <el-input-number v-model="createForm.orderID" :min="1" />
              </div>
            </div>
            <div>
              <label class="mini-note">店铺 ID（与商家端一致，商家才能查到此评价）</label>
              <el-input-number v-model="createForm.storeID" :min="1" style="width:180px" />
            </div>
            <div>
              <label class="mini-note">综合评分</label>
              <el-rate v-model="createForm.score" :max="5" />
            </div>
            <div class="dense-form-grid">
              <div>
                <label class="mini-note">服务评分</label>
                <el-rate v-model="createForm.serviceScore" :max="5" />
              </div>
              <div>
                <label class="mini-note">物流评分</label>
                <el-rate v-model="createForm.expressScore" :max="5" />
              </div>
            </div>
            <div>
              <label class="mini-note">评价内容</label>
              <el-input v-model="createForm.content" type="textarea" :rows="4" placeholder="分享你的购物体验..." />
            </div>
            <div class="dense-form-grid">
              <div>
                <label class="mini-note">晒单图片</label>
                <LocalImageUploader v-model="createForm.picInfo" />
              </div>
              <div>
                <label class="mini-note">视频信息</label>
                <el-input v-model="createForm.videoInfo" type="textarea" :rows="2" />
              </div>
            </div>
            <div class="inline-actions">
              <el-switch v-model="createForm.anonymous" inline-prompt active-text="匿名" inactive-text="实名" />
              <el-button type="primary" :loading="loading" @click="handleCreateReview">立即发表</el-button>
            </div>
          </div>
        </el-card>

        <el-card class="commerce-card table-card" shadow="never">
          <div class="section-header">
            <div>
              <h3 class="section-title">我的评价广场</h3>
              <p class="section-subtitle">更像电商评价流而不是纯表格</p>
            </div>
            <el-tag type="info" round>用户查询</el-tag>
          </div>
          <div class="table-toolbar">
            <el-input-number v-model="userQuery.userID" :min="1" />
            <el-input-number v-model="userQuery.page" :min="1" />
            <el-input-number v-model="userQuery.pageSize" :min="1" :max="100" />
            <el-button type="primary" :loading="loading" @click="handleLoadUserReviews">刷新列表</el-button>
          </div>
          <div v-if="reviewFeed.length" class="review-card-list">
            <article v-for="row in reviewFeed" :key="row.reviewID" class="review-card">
              <div class="placeholder-image review-thumb">
                <span>晒单图占位</span>
              </div>
              <div>
                <div class="review-card-header">
                  <div>
                    <p class="review-card-title">评价单号 #{{ row.reviewID }}</p>
                    <p class="review-card-meta">{{ formatTime(row.createAt) }}</p>
                  </div>
                  <el-tag round>{{ statusLabel(row.status) }}</el-tag>
                </div>
                <el-rate :model-value="row.score" disabled />
                <p class="review-card-content">{{ row.content || '暂无文字评价内容，可在详情编辑区补充。' }}</p>
                <div class="review-card-tags">
                  <span>订单 {{ row.orderID }}</span>
                  <span>服务 {{ row.serviceScore }} 分</span>
                  <span>物流 {{ row.expressScore }} 分</span>
                  <span>{{ row.hasReply ? '商家已回复' : '待商家回复' }}</span>
                </div>
              </div>
            </article>
          </div>
          <el-empty v-else description="还没有评价记录，先创建一条体验一下" />
          <div class="table-footer">共 {{ userTotal }} 条</div>
        </el-card>
      </div>

      <div class="panel-grid">
        <el-card class="commerce-card table-card" shadow="never">
          <div class="section-header">
            <div>
              <h3 class="section-title">订单评价追踪</h3>
              <p class="section-subtitle">适合模拟订单详情页里的全部买家评价</p>
            </div>
            <el-tag type="warning" round>订单查询</el-tag>
          </div>
          <div class="table-toolbar">
            <el-input-number v-model="orderQuery.orderID" :min="1" />
            <el-input-number v-model="orderQuery.page" :min="1" />
            <el-input-number v-model="orderQuery.pageSize" :min="1" :max="100" />
            <el-button type="primary" :loading="loading" @click="handleLoadOrderReviews">查看订单评价</el-button>
          </div>
          <el-table :data="orderList" stripe>
            <el-table-column prop="reviewID" label="评价单" min-width="110" />
            <el-table-column prop="userID" label="用户" min-width="100" />
            <el-table-column prop="score" label="评分" min-width="80" />
            <el-table-column prop="hasReply" label="商家回复" min-width="100" />
            <el-table-column prop="content" label="评价内容" min-width="260" show-overflow-tooltip />
          </el-table>
          <div class="table-footer">共 {{ orderTotal }} 条</div>
        </el-card>

        <el-card class="commerce-card" shadow="never">
          <div class="section-header">
            <div>
              <h3 class="section-title">详情编辑工作台</h3>
              <p class="section-subtitle">模拟“我的订单 - 评价详情 - 继续编辑”链路</p>
            </div>
            <el-tag type="danger" round>可编辑</el-tag>
          </div>
          <div class="table-toolbar">
            <el-input-number v-model="detailReviewID" :min="1" placeholder="reviewID" />
            <el-button type="primary" :loading="loading" @click="handleLoadDetail">加载详情</el-button>
          </div>
          <el-descriptions v-if="detail" :column="2" border class="detail-box">
            <el-descriptions-item label="ReviewID">{{ detail.reviewID }}</el-descriptions-item>
            <el-descriptions-item label="用户">{{ detail.userID }}</el-descriptions-item>
            <el-descriptions-item label="订单">{{ detail.orderID }}</el-descriptions-item>
            <el-descriptions-item label="状态">{{ statusLabel(detail.status) }}</el-descriptions-item>
            <el-descriptions-item label="创建时间" :span="2">{{ formatTime(detail.createAt) }}</el-descriptions-item>
            <el-descriptions-item label="内容" :span="2">{{ detail.content }}</el-descriptions-item>
          </el-descriptions>
          <el-empty v-else description="输入 reviewID 加载一条评价详情" />

          <el-divider />
          <div class="dense-form">
            <div class="dense-form-grid">
              <div>
                <label class="mini-note">Review ID</label>
                <el-input-number v-model="editForm.reviewID" :min="1" />
              </div>
              <div>
                <label class="mini-note">用户 ID</label>
                <el-input-number v-model="editForm.userID" :min="1" />
              </div>
            </div>
            <div>
              <label class="mini-note">综合评分</label>
              <el-rate v-model="editForm.score" :max="5" />
            </div>
            <div class="dense-form-grid">
              <div>
                <label class="mini-note">服务评分</label>
                <el-rate v-model="editForm.serviceScore" :max="5" />
              </div>
              <div>
                <label class="mini-note">物流评分</label>
                <el-rate v-model="editForm.expressScore" :max="5" />
              </div>
            </div>
            <div>
              <label class="mini-note">评价内容</label>
              <el-input v-model="editForm.content" type="textarea" :rows="4" />
            </div>
            <div class="dense-form-grid">
              <div>
                <label class="mini-note">晒单图片</label>
                <LocalImageUploader v-model="editForm.picInfo" />
              </div>
              <div>
                <label class="mini-note">视频信息</label>
                <el-input v-model="editForm.videoInfo" type="textarea" :rows="2" />
              </div>
            </div>
            <div class="inline-actions">
              <el-button type="primary" :loading="loading" @click="handleUpdateReview">保存修改</el-button>
              <el-popconfirm title="确认删除该评价吗？" @confirm="handleDeleteReview">
                <template #reference>
                  <el-button type="danger" :loading="loading">删除评价</el-button>
                </template>
              </el-popconfirm>
            </div>
          </div>
        </el-card>
      </div>
    </div>
  </AppShell>
</template>
