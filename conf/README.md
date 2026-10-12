# conf 配置占位符解析

`conf` 包为 `viper.UnmarshalKey` / `viper.Unmarshal` 提供了 DecodeHook 扩展：配置文件中的字符串值可以使用占位符，加载配置时自动解析为实际值。典型用途是避免在配置文件中明文存放密码、密钥等敏感信息。

## 占位符语法

| 占位符 | 说明 | 默认值 |
| --- | --- | --- |
| `${secret: xxx}` | AES-CBC 加密密文，自动解密为明文 | 不支持 |
| `${env: NAME[:default]}` | 读取环境变量 | 可选 |
| `${file: /path}` | 读取文件内容 | 不支持 |

通用规则：

- `secret` / `env` / `file` 关键字**不区分大小写**（`${SECRET: ...}`、`${Env: ...}` 均可），但变量名、文件路径、密文内容本身大小写敏感。
- 占位符可出现在字符串的任意位置，同一字符串中可内嵌、混用多个占位符。
- 关键字与内容之间、内容首尾允许空白字符；内容中不允许出现 `{` 或 `}`。
- 任一占位符解析失败，整个配置加载报错——错误在启动阶段暴露，而不是静默变成空值。

## 快速开始

以数据库密码为例，三步完成密文配置：

1. 生成密文（CLI 与服务端解密使用相同的密钥/IV 规则）：

   ```bash
   $ ctl encrypt my-password
   qg5lFlIxLWKUtXUKi9FVoLfmiTNDTRVjQfOh7k/ti+o=
   ```

2. 写入配置文件：

   ```toml
   [DB]
   User = "root"
   Password = "${secret: qg5lFlIxLWKUtXUKi9FVoLfmiTNDTRVjQfOh7k/ti+o=}"
   ```

3. 正常加载配置，`Password` 字段自动得到明文 `my-password`，业务代码无需任何改动。

## 各占位符详解

### ${secret: xxx} — 密文自动解密

默认解密器使用 AES-CBC（`pkg/safety.CBCDecrypter`），密钥与 IV 通过环境变量提供：

```bash
export SERVER_SECRET_KEY="6kY8uyQEl6XV37KR"   # 16/24/32 字节
export SERVER_SECRET_IV="E5ZKizMERreD3HX8"    # 16 字节
```

未设置环境变量时使用内置默认值（key `6kY8uyQEl6XV37KR`，iv `E5ZKizMERreD3HX8`），零配置即可加解密；两个变量独立判断，只设置其中一个时另一个仍用默认值。

生成密文的方式：

```bash
ctl encrypt my-password          # CLI 命令
```

```go
cipher, _ := conf.EncryptSecret("my-password")   // 代码方式，密钥规则相同
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
Uri = "mongodb://${env: MONGO_USER:root}:${secret: xxxx}@${env: MONGO_HOST:localhost}:27017/admin"
```

## 代码接入

`conf.GetClient`、`conf.NewConfig` 以及 `redis` / `db` / `db/mongodb` 模块的 `Initialize` 均已接入该能力，直接使用即可。

新代码使用 `conf.UnmarshalKey` / `conf.Unmarshal` 替代 viper 同名函数：

```go
var cfg MyConfig
if err := conf.UnmarshalKey("MyService", &cfg); err != nil {
    return err
}
```

如果必须直接调用 viper，也可手动传入 Hook：

```go
viper.UnmarshalKey("MyService", &cfg, conf.DecodeHook())
```

此外 `conf.ResolveValue(text)` 可单独使用，直接解析任意字符串中的占位符。

## 实现原理

核心代码位于 `conf/decode.go`，整体链路如下：

```
配置文件字符串
      │
      ▼
viper.UnmarshalKey(key, &cfg, conf.DecodeHook())
      │
      ▼
ValueDecodeHookFunc()          // mapstructure.DecodeHookFunc
      │  仅处理 string → string 的字段，其余类型直接透传
      ▼
ResolveValue(text)
      │  正则 \$\{(?i:(secret|env|file)):([^{}]+?)\} 匹配所有占位符并逐个替换
      ▼
resolvePlaceholder(kind, spec) // 按关键字分发（kind 先 ToLower）
      ├─ secret → secretDecryptor（默认 AES-CBC 解密）
      ├─ env    → os.LookupEnv，未设置时用默认值
      └─ file   → os.ReadFile，并去除末尾换行符
```

要点：

- `DecodeHook()` 通过 `mapstructure.ComposeDecodeHookFunc` 把占位符解析 Hook 与 viper 默认的 `StringToTimeDurationHookFunc`、`StringToSliceHookFunc` 组合，避免覆盖 viper 原有行为（`viper.DecodeHook` 是整体替换默认 Hook 的）。
- 加解密共用 `secretKeyIV()` 获取密钥/IV（环境变量优先，空则回退内置默认值），保证 `ctl encrypt` 生成的密文与服务端解密规则一致。

## 注意事项

- 占位符仅对 **string 类型字段**生效，目标是 int、bool 等类型的字段不会触发解析。
- 内置默认密钥/IV 随源码公开，仅适用于开发测试；使用默认密钥加密的密文不具备真正的保密性，生产环境务必通过环境变量覆盖。
- `SERVER_SECRET_KEY` / `SERVER_SECRET_IV` 属于敏感信息，请通过安全的途径（如密钥管理系统、容器 secret 挂载）注入，不要写入配置文件或代码仓库。
