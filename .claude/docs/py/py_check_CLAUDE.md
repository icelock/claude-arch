# CLAUDE.md

本文件是 Python 的检查命令与语言级自查项，供 `py_arch_layer_CLAUDE.md` 和 `py_arch_onion_CLAUDE.md` 共用。架构自查项在对应的架构文件中，通用完成定义见 `../common/definition_of_done_CLAUDE.md`。

## 检查命令

```bash
ruff check .
ruff format --check .
mypy .        # 或 pyright
pytest        # 全量，不只跑改动目录
```

## 语言级自查

以下规则工具拦不住，提交前逐项确认：

- 金额构造与舍入符合 `py_lang_CLAUDE.md`：`Decimal` 从字符串或整数构造，`quantize` 显式指定舍入。
- 事务内没有远程调用；数据库与消息的一致性用 outbox 或提交后发送。
- async 代码中没有阻塞调用（`time.sleep`、`requests`、同步数据库驱动、大量 CPU 计算）。
- 锁内没有 IO、回调或慢操作。
- 日志和异常信息中没有密钥、token、密码。
- 有界队列满时的策略已写明；不能丢的数据只会拒绝受理或熔断。
