测试框架使用说明 / Test Framework README

概览
-----
本说明文档描述仓库中基于 `test/` 目录的集成测试/端到端测试框架的使用方法、用例组织规则、自动生成器用法以及常见故障排查方法。

目标读者
---------
- 开发者：快速为后端 API 方法生成集成测试用例骨架，并运行验证。
- 测试工程师：准备 DB 数据（CSV）、校验返回与数据库，执行并调试测试用例。

目录结构（约定）
-----------------
- test/
  - harness/                # 测试框架代码（generator、工具、基类等）
    - gen_case.go           # 生成器（手动执行：go run test/harness/gen_case.go ...）
  - <Method>/               # 每个被测方法的顶级目录（例如 handleLogin、handleRegister）
    - <method>_test.go      # harness 风格的测试 wrapper（由 generator 创建/更新）
    - case01/               # 用例编号目录（可以有多个 caseNN）
      - case.yml            # 用例描述（请求/期望/校验规则）
      - PrepareData/        # 可选：放置准备数据库用的 CSV 文件（表名.csv）
      - CheckData/          # 可选：放置校验数据库用的 CSV 文件（表名.csv）

设计理念与执行流程
-------------------
1. 为某个 API 方法生成测试骨架（generator）：会生成 `test/<Method>/case01/case.yml` 与 `test/<Method>/<Method>_test.go`（harness wrapper）。
2. 准备数据库数据（可选）：在 `PrepareData` 中放置 `table.csv`（格式见下）。
3. 运行测试：`go test ./test/<Method> -run Test<Method> -v`。
4. 测试执行流程（高层）：
   - harness 启动临时测试环境（内存/临时 DB、httptest Server）；
   - 如果有 PrepareData，会将 CSV 中定义的记录插入 DB；
   - 根据 `case.yml` 发起 HTTP 请求到测试 API；
   - 验证 HTTP 状态与响应 body（case.yml 中的 expect）；
   - 根据 CheckData 的 CSV 对 DB 进行查询校验；
   - 执行完后清理由 flag=c 表示需要删除的 DB 数据（或由框架统一清理）。

case.yml（用例描述）
-------------------
case.yml 用 YAML 描述一个 API 测试用例，示例结构如下：

```yaml
name: handleLogin case01
request:
  method: POST
  path: /api/login
  headers: {}
  body:
    email: testuser@example.com
    password: password123
expect:
  status: 200
  body:
    email: testuser@example.com
    token: EXISTS:true
```

字段说明：
- name: 用例的标识（可随意）。
- request:
  - method: HTTP 方法（GET/POST/PUT等）。
  - path: 测试时请求的 URL path（必须与项目中注册的路由一致，例如 `/api/login`）。
  - headers: 可选的 HTTP 头部。
  - body: 请求 JSON body（当 method 为 POST/PUT 等时）。
- expect:
  - status: 期望的 HTTP 状态码。
  - body: 对响应 JSON 的断言，断言表达形式：
    - 精确匹配值（例如 email: test@example.com）
    - EXISTS:true 表示该字段必须存在且非空
    - "*" 表示忽略具体值（仅关注字段存在或结构）

CSV 数据格式（PrepareData / CheckData）
---------------------------------------
- 存放在对应用例目录的 `PrepareData/` 或 `CheckData/` 内。
- 文件命名规则：以表名命名，例如 `users.csv`（不要包含 table 字段）。
- CSV 内容格式（无 header，或以 header `column,value,flag` 可被忽略）：
  - 每行 3 列：column,value,flag
  - column: 列名
  - value: 要插入或校验的值，使用 `*` 表示非空校验
  - flag: 标记用途，含义：
    - c : 用作 WHERE 条件（用于查找/删除）
    - Y : 需要校验（在 CheckData 中，表示该列的值需与期望匹配）
    - n : 跳过（忽略该列）

示例 `PrepareData/users.csv`：
```
column,value,flag
email,testuser@example.com,c
password_hash,dummyhash,Y
otp_secret,*,Y
otp_verified,0,Y
```

生成器 `gen_case.go` 用法
------------------------
- 位置：`test/harness/gen_case.go` （带 `//go:build ignore`，不会随 `go test` 被构建）
- 用法（在项目根目录执行）：

```bash
# 生成名为 Server.handleLogin 的用例（默认 case01）
go run test/harness/gen_case.go Server.handleLogin

# 或直接用函数名（如果无需接收者区分）
go run test/harness/gen_case.go handleLogin

# 指定用例目录名（例如 case02）
go run test/harness/gen_case.go handleLogin case02
```

生成器会：
- 在 `test/<Func>/caseName/case.yml` 写入推断的 request/expect（若能解析出路由会写入真实路径，如 `/api/login`）；
- 创建空目录 `PrepareData` 和 `CheckData`；
- 在 `test/<Func>/<Func>_test.go` 写入 harness 风格的测试 wrapper（含 `@Target` 和 `@RunWith` 注解）。

注意：生成器做启发式 AST 解析，常见场景有效；如果没有识别到想要的字段或路由，请手动编辑 `case.yml`。

运行测试
--------
- 单个用例包运行（推荐）：

```bash
go test ./test/handleLogin -run TestHandleLogin -v
```

- 运行所有 test 下的测试（慎用，时间较长）：

```bash
go test ./test/... -v
```

调试失败 & 常见问题
-----------------
1) 404 页面未找到：
   - 常见原因：`case.yml` 中的 `request.path` 与实际路由不一致。检查 `api/server.go` 中注册的路径（注意 group 前缀，例如 `/api`）。
   - 解决：手动把 `case.yml` 的 path 修为正确值（例如 `/api/login`）。

2) 数据库校验失败（CheckData 报错）：
   - 检查 CSV 文件的 `column,value,flag` 格式是否正确。
   - flag=c 表示 WHERE 条件，框架会用这些列做查询，确保 WHERE 能唯一定位期望行。
   - value 使用 `*` 表示非空校验。

3) 返回 body 与期望不匹配：
   - 框架默认只对 `expect.body` 中列出的字段做检查；如果实际返回中有额外字段，框架不会因为额外字段而失败，但如果期望值为通配或存在检查，框架会验证。
   - 当某些字段返回是动态值（比如 otp_secret），请使用 `EXISTS:true` 或 `*` 来放宽断言。

4) 生成器未生成 wrapper 或生成的不在预期目录：
   - 使用 `go run test/harness/gen_case.go Server.handleLogin`（确保传入的是接收者类型名而不是实例变量名）。
   - 生成器会把 harness wrapper 写入 `test/<Func>/<Func>_test.go`（父目录），而把 `case.yml` 放在 `test/<Func>/<caseName>/case.yml`。

开发者提示
---------
- 若你修改了生成器 `gen_case.go`，因为它带 `//go:build ignore`，请通过 `go run` 单独执行，而不是 `go test ./...`。
- 增强建议：把常用的 `-req`/`-expect` 覆盖参数放到生成器，以便使用命令行快速生成精确的 `case.yml`，我可以帮你实现。

示例工作流（从生成到运行）
-----------------------
1. 自动生成用例骨架：
```bash
go run test/harness/gen_case.go Server.handleVerifyOTP
```
2. 打开并编辑 `test/handleVerifyOTP/case01/case.yml`，根据需要调整 `request.path`（确保为 `/api/verify-otp`），与期望的 body 字段。
3. 如果需要准备 DB：将 `users.csv` 等放入 `test/handleVerifyOTP/case01/PrepareData/`，格式见上文。
4. 运行测试并查看输出：
```bash
go test ./test/handleVerifyOTP -run TestHandleVerifyOTP -v
```
5. 如果失败，根据失败信息定位：HTTP status、response body、或 DB 校验失败，然后相应修改 `case.yml` 或 CSV。

结束语
------
本 README 旨在帮助你快速上手并维护基于 `test/` 目录的集成测试用例。如果你希望我把生成器增强为支持命令行覆盖、自动生成 CSV 模板，或把测试执行集成进 CI（GitHub Actions），告诉我你倾向的优先级，我会直接实现并验证。

