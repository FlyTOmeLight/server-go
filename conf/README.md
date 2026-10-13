# conf 配置占位符解析

`conf` 包在加载配置时解析字符串中的占位符，并保存已解析的有效值。典型用途是避免在配置文件中明文存放密码、密钥等敏感信息。直接使用 Viper 的调用方也可采用 `DecodeHook` 扩展。

## 占位符语法

| 占位符 | 说明 | 默认值 |
| --- | --- | --- |
| `${secret: xxx}` | AES-GCM 加密密文，自动解密为明文 | 不支持 |
| `${env: NAME[:default]}` | 读取环境变量 | 可选 |
| `${file: /path}` | 读取文件内容 | 不支持 |

通用规则：

- `secret` / `env` / `file` 关键字**不区分大小写**（`${SECRET: ...}`、`${Env: ...}` 均可），但变量名、文件路径、密文内容本身大小写敏感。
- 占位符可出现在字符串的任意位置，同一字符串中可内嵌、混用多个占位符。
- 关键字与内容之间、内容首尾允许空白字符；内容中不允许出现 `{` 或 `}`。
- 任一占位符解析失败，整个配置加载报错——错误在启动阶段暴露，而不是静默变成空值。

## 快速开始

以数据库密码为例，三步完成密文配置：

1. 生成密文（先显式设置密钥，CLI 与服务端使用相同密钥）：

   ```bash
   export SERVER_SECRET_KEY="$(openssl rand -hex 16)" # 32 字节随机密钥
   ctl encrypt my-password
   # 输出格式为 v1:<base64>，保存实际输出
   ```

2. 写入配置文件：

   ```toml
   [DB]
   User = "root"
   Password = "${secret: v1:<ctl encrypt 的实际输出内容>}"
   ```

3. 正常加载配置，`Password` 字段自动得到明文 `my-password`，业务代码无需任何改动。

## 各占位符详解

### ${secret: xxx} — 密文自动解密

默认解密器使用标准库 AES-GCM 认证加密。`SERVER_SECRET_KEY` 必须显式提供 16、24 或 32 字节密钥；未设置或长度无效时，加密和解密均返回错误。随机生成密钥后，通过运行环境的秘密配置同时注入 CLI 和服务端：

```bash
export SERVER_SECRET_KEY="$(openssl rand -hex 16)" # 32 字节；生成一次并保存
```

密文格式为 `v1:<base64>`。Base64 内容依次包含随机 nonce、密文和认证标签。每次加密生成新的 nonce，因此同一明文的输出不同。解密验证标签，篡改数据或错误密钥均返回错误。无需设置 IV。

未带版本的旧 CBC 密文及其他版本均明确拒绝。请使用原始明文和新密钥重新运行 `ctl encrypt`；不存在默认密钥或旧格式回退。

生成密文的方式：

```bash
ctl encrypt my-password          # CLI 命令
```

```go
ciphertext, err := conf.EncryptSecret("my-password")
if err != nil {
    return err
}
_ = ciphertext
```

如需接入 KMS 等其他解密方案，可替换解密实现：

```go
conf.SetSecretDecryptor(func(ciphertext string) (string, error) {
    return myKMS.Decrypt(ciphertext)
})
```

### ${env: NAME[:default]} — 读取环境变量

```toml
[HTTPServer]
Listen = "${env: LISTEN_ADDR::8080}"   # 环境变量未设置时使用 :8080
LogLevel = "${env: LOG_LEVEL}"          # 未设置且无默认值 → 报错
```

- 以第一个冒号切分变量名与默认值，因此默认值本身可以包含冒号（如 `${env: ADDR:127.0.0.1:8080}` → 默认值 `127.0.0.1:8080`）。
- 环境变量已设置（即使为空字符串）时优先使用环境变量的值，默认值不生效。

### ${file: /path} — 读取文件内容

```toml
[Redis]
Password = "${file: /run/secrets/redis-password}"
```

读取文件全部内容作为值，并自动去除末尾的 `\r\n`（兼容密钥文件以换行结尾的常见写法）。文件不存在或读取失败会报错，不支持默认值。

### 混用与内嵌

```toml
[MongoDB]
Uri = "mongodb://${env: MONGO_USER:root}:${secret: v1:<实际密文>}@${env: MONGO_HOST:localhost}:27017/admin"
```

## 代码接入

使用独立的 `conf.Manager` 加载完整文件。默认路径为 `conf/server/config.toml`，可由 `SERVER_CONFIG_PATH` 指定。`AppName`、非零 `AppCode` 和 `RunMode`（`debug`、`test`、`release`）必填。资源配置从实例快照解析：

```go
manager, err := conf.NewManager("")
if err != nil {
    return err
}
var cfg MyConfig
if err := manager.Snapshot().UnmarshalKey("MyService", &cfg); err != nil {
    return err
}
```

如果必须直接调用 viper，也可手动传入 Hook：

```go
viper.UnmarshalKey("MyService", &cfg, conf.DecodeHook())
```

此外 `conf.ResolveValue(text)` 可单独使用，直接解析任意字符串中的占位符。

程序化输入使用 `conf.FromSettings` 解析占位符；直接传给 `NewManagerFromConfig` 的 `Config` 视为已解析的有效值。快照的 `Unmarshal` / `UnmarshalKey` 仅完成类型转换，保留 Viper 默认的 duration 和字符串切片转换。

## 动态更新与所有权

每次加载使用独立解析器，文件中删除的键不会残留。所有配置段的占位符在加载时解析一次，包括业务段、嵌套字段和数组。解析结果中的占位符文本作为普通内容保留，不会再次展开。`Snapshot()` 返回独立副本；修改副本不会改变当前版本。同一快照不会重新读取环境变量、秘密文件或解密器。

`Reload` 重新读取文件和占位符来源，按有效值比较变更；即使配置文件中的占位符文本没有变化，来源值变化也会进入订阅者校验。`Update` / `Reload` 先校验所有订阅者，成功后发布并执行提交动作；失败保留原版本。准备和提交回调不得再次调用 Manager 的更新或订阅方法。

`Watch(ctx, onError)` 监听所属配置文件，不监听 `${file: ...}` 引用的文件；来源文件变化后需显式调用 `Reload` 或重新加载进程。入口在退出时调用 `Close(ctx)` 取消并等待监听结束。`Bind` 原子完成当前版本的初始化和后续订阅；普通 `Subscribe` 只接收后续变化，返回解除订阅函数。

HTTPHost 支持动态更新路由、中间件、Feature、FakeAuth 和错误展示模式，不重启业务 worker。监听地址、HTTP 协议、超时、Header 限制、AppCode 和基础设施配置需要重启，Host 会返回 `ErrRestartRequired` 并保留当前版本。DB、Redis、MongoDB 和客户端由装配代码显式构造及关闭，不进行隐式连接替换。

## 实现原理

核心代码位于 `conf/decode.go`，整体链路如下：

```
配置文件字符串
      │
      ▼
Load / FromSettings
      │
      ▼
递归读取配置段与数组中的字符串
      │
      ▼
ResolveValue(text)
      │  正则 \$\{(?i:(secret|env|file)):([^{}]+?)\} 匹配所有占位符并逐个替换
      ▼
resolvePlaceholder(kind, spec) // 按关键字分发（kind 先 ToLower）
      ├─ secret → secretDecryptor（默认 AES-GCM 解密）
      ├─ env    → os.LookupEnv，未设置时用默认值
      └─ file   → os.ReadFile，并去除末尾换行符
      │
      ▼
保存有效值 → Snapshot().Unmarshal / UnmarshalKey
```

要点：

- `DecodeHook()` 通过 `mapstructure.ComposeDecodeHookFunc` 把占位符解析 Hook 与 viper 默认的 `StringToTimeDurationHookFunc`、`StringToSliceHookFunc` 组合，避免覆盖 viper 原有行为（`viper.DecodeHook` 是整体替换默认 Hook 的）。
- 加解密共用 `secretCipher()` 读取显式密钥并构造 AES-GCM，保证 `ctl encrypt` 与服务端使用相同格式。

## 注意事项

- `Load` / `FromSettings` 先解析所有字符串，再转换为目标字段类型，因此数字、bool 等字段也可使用字符串占位符。直接使用 `DecodeHook()` 时，占位符仅对 string → string 转换生效。
- `SERVER_SECRET_KEY` 属于敏感信息，请通过安全的途径（如密钥管理系统、容器 secret 挂载）注入，不要写入配置文件或代码仓库。
