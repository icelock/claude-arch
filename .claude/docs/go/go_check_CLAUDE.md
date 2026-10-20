# CLAUDE.md

本文件是 Go 的检查命令与语言级自查项，供 `go_arch_layer_CLAUDE.md` 和 `go_arch_onion_CLAUDE.md` 共用。架构自查项在对应的架构文件中，通用完成定义见 `../common/definition_of_done_CLAUDE.md`。

## 检查命令

```bash
go vet ./...
golangci-lint run --enable bodyclose,rowserrcheck,sqlclosecheck,noctx
go test -race ./...
```

`golangci-lint` 默认已含 `errcheck`、`govet`、`staticcheck`。

## 语言级自查

以下规则 lint 拦不住，提交前逐项确认：

- 金额构造与舍入符合 `go_lang_CLAUDE.md`：不从浮点构造，除法显式指定精度和舍入方式，比较用 `Cmp`/`Equal`。
- 事务内没有远程调用；数据库与消息的一致性用 outbox 或提交后发送。
- 锁内没有 IO、回调或获取其他锁的操作。
- 日志和错误信息中没有密钥、token、密码。
- 有界 channel 满时的策略已写明并处理 `Full`；不能丢的数据只会拒绝受理或熔断。
- 回调类型没有既实现下层接口、又持有该下层客户端；cmd 中的装配关系无环。
