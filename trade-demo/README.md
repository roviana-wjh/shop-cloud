# Trade Demo：Kratos + gRPC 编排式 Saga

商品、库存、订单、模拟支付与 Saga 编排器是五个独立进程。服务使用 Kratos v2 管理生命周期和 HTTP/gRPC Server，服务间只通过生成的 Protobuf gRPC 客户端调用。无需启动 MySQL、Consul、Kafka 或 K8s；本地 JSON 数据文件仍由各服务独立持有，原有评价服务不受影响。

## 运行

需要 Go 1.25+。一键脚本需要 Windows PowerShell 7.4+（使用 `Start-Process -Environment`）。在仓库根目录运行：

```powershell
cd trade-demo
.\scripts\start.ps1
# 停止时：
.\scripts\stop.ps1
```

也可以打开五个终端，在本目录分别执行 `go run . -service=product`、`inventory`、`order`、`payment`、`saga`。启动顺序不限；全部服务就绪后再调用业务接口。

| 进程 | HTTP 入口 | gRPC 地址 | 数据文件 |
|---|---|---|---|
| product | localhost:8031 | localhost:9031 | data/product.json |
| inventory | localhost:8032 | localhost:9032 | data/inventory.json |
| order | localhost:8033 | localhost:9033 | data/order.json |
| payment | localhost:8034 | localhost:9034 | data/payment.json |
| saga | localhost:8035 | localhost:9035 | data/saga.json |

HTTP 地址使用 `TRADE_ADDR`、gRPC 地址使用 `TRADE_GRPC_ADDR`，数据文件使用 `TRADE_DATA_FILE` 覆盖。服务间直连地址分别使用 `PRODUCT_GRPC_ADDR`、`INVENTORY_GRPC_ADDR`、`ORDER_GRPC_ADDR`、`PAYMENT_GRPC_ADDR`、`SAGA_GRPC_ADDR`，值为 `host:port`。旧的 `*_URL` 配置已移除。所有进程提供 HTTP `GET /healthz`。

## 验证支付与补偿

```powershell
Invoke-RestMethod http://localhost:8031/products
$created = Invoke-RestMethod -Method Post -Uri http://localhost:8033/orders -ContentType 'application/json' -Body '{"userId":"demo-user","productId":"tea","quantity":2}'
$created
Invoke-RestMethod -Method Post -Uri "http://localhost:8034/payments/$($created.saga.paymentId)/simulate" -ContentType 'application/json' -Body '{"result":"success"}'
Invoke-RestMethod "http://localhost:8033/orders/$($created.order.id)"
Invoke-RestMethod "http://localhost:8035/sagas/$($created.saga.id)"
Invoke-RestMethod http://localhost:8032/inventory
```

成功时订单为 `PAID`，Saga 为 `COMPLETED`。另建一笔订单，把模拟结果改为 `fail`：库存预留被释放，订单为 `CANCELLED`，Saga 为 `FAILED`。重复提交相同支付结果不会重复确认或释放库存，提交相反结果会返回 409。

首次空库含 `tea`（20 件，1290 分）和 `mug`（8 件，3990 分）。已有 JSON 文件会继续使用，不重置库存或订单。金额单位为分；Kratos 使用标准 Protobuf JSON 编码，因此 HTTP 响应中的 int64（金额、数量、库存）表现为字符串；成功的 HTTP POST 默认返回 200。

## 调用链与分层

```text
HTTP 创建订单 → Kratos OrderService → biz.CreateOrder
  → gRPC ProductService.GetProduct：获取商品价格快照
  → 保存待支付订单
  → gRPC SagaService.StartSaga
      → gRPC OrderService.GetOrder
      → gRPC ProductService.GetProduct
      → gRPC InventoryService.ReserveStock
      → gRPC PaymentService.CreatePayment
      → WAITING_FOR_PAYMENT

HTTP 模拟支付 → Kratos PaymentService → 保存支付结果
  → gRPC SagaService.NotifyPaymentResult
      → gRPC PaymentService.GetPayment：核对真实支付状态
      → 成功：ConfirmReservation → MarkOrderPaid → COMPLETED
      → 失败：ReleaseReservation → CancelOrder → FAILED
```

- `api/trade/v1/trade.proto`：五个服务的强类型消息与 RPC，HTTP 注解和生成代码。
- `internal/server`：注册每个进程自己的 Kratos HTTP/gRPC 服务，恢复与 tracing 中间件。
- `internal/service`：Proto 参数与领域模型转换，调用用例。
- `internal/biz`：商品/库存/订单/支付规则、Saga 状态推进与补偿；依赖 Repository、Remote 接口。
- `internal/data`：独立 JSON Repository 与生成的 gRPC 客户端、连接生命周期。
- `internal/mapper`：领域模型与 Proto 消息转换。
- `main.go`：按 `-service` 组装依赖，由 `kratos.App` 启停双协议服务。

为了保持本地启动简单，五个进程暂共用一个 Go module 和二进制；每个进程只注册自己的 RPC，数据和客户端依赖相互隔离。本次采用显式依赖注入，尚未使用 Wire。

## 接口边界

HTTP 保留：商品列表/详情、库存查询、订单创建/查询、支付查询/模拟支付、Saga 查询。旧的 `/internal/*`、`POST /sagas`、`POST /callbacks/payment` HTTP 路由已删除；内部写操作仅有 gRPC 接口，具体方法见 Proto。gRPC 当前为本地明文连接，没有鉴权，不应直接用于公网真实支付。

## 生成接口与验证

生成文件已提交，普通构建不需要 protoc。修改 Proto 后需要 protoc 及插件：

```powershell
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.34.1
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.4.0
go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@v2.0.0-20260404020628-f149714c1d54
# 将 Go bin 加到 PATH，安装 protoc 后执行：
.\scripts\generate.ps1
go test ./...
go build -buildvcs=false ./...
```

集成测试启动五个实际 Kratos gRPC Server，使用生成的客户端验证成功支付、失败补偿、重复回调/冲突结果、库存不足与取消，以及生成的 HTTP 入口和 JSON 持久化；使用随机端口和临时文件，不污染演示数据。

## 当前限制

仍是单机、单副本学习版：JSON 文件不可由多个同角色进程共享；没有 outbox、后台自动恢复/重试、超时关单、真实支付和评价资格联动。记录了 Saga 步骤不等于重启后能自动执行剩余步骤。

支付成功后下游暂时不可用，会保持 `CONFIRMING`，再次提交相同成功结果可继续正向确认，不能自动退款或释放库存。支付失败补偿出错则标为 `COMPENSATION_REQUIRED`，再次提交相同失败结果可以重试补偿。预留/创建支付 RPC 发生结果不确定的网络错误会标为 `RECOVERY_REQUIRED`，保留资源与记录供后续恢复设计，避免把超时当作业务失败而错误补偿；本版不自动恢复该状态。未实现创建订单请求的全链路幂等，请勿将重复创建订单当作重试原订单。
