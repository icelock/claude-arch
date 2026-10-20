# CLAUDE.md

本文件是 Java 的检查命令与语言级自查项，供 `java_arch_layer_CLAUDE.md` 和 `java_arch_onion_CLAUDE.md` 共用。架构自查项在对应的架构文件中，通用完成定义见 `../common/definition_of_done_CLAUDE.md`。

## 检查命令

```bash
mvn test
```

## 语言级自查

以下规则工具拦不住，提交前逐项确认：

- 金额构造与舍入符合 `java_lang_CLAUDE.md`：没有 `new BigDecimal(double)`，除法显式指定 scale 和 `RoundingMode`。
- 事务内没有远程调用；数据库与消息的一致性用 outbox 或 `afterCommit`。
- 锁内没有 IO、回调或获取其他锁的操作。
- 日志和异常信息中没有密钥、token、密码。
- 有界队列的满队列策略已写明；不能丢的数据只会拒绝受理或熔断。
