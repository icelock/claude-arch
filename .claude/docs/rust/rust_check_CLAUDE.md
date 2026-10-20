# CLAUDE.md

本文件是 Rust 的检查命令与语言级自查项，供 `rust_arch_layer_CLAUDE.md` 和 `rust_arch_onion_CLAUDE.md` 共用。架构自查项在对应的架构文件中，通用完成定义见 `../common/definition_of_done_CLAUDE.md`。

## 检查命令

```bash
cargo fmt --check
cargo clippy --all-targets -- -D warnings    # workspace 用 --workspace
cargo test                                   # 多 crate / workspace 用 --workspace
```

在 `Cargo.toml` 中用 lint 把文字约束变成编译检查（workspace 写 `[workspace.lints.clippy]`，各 crate 写 `[lints] workspace = true`）：

```toml
[lints.clippy]
unwrap_used = "deny"
expect_used = "deny"
panic = "deny"
todo = "deny"
unimplemented = "deny"
dbg_macro = "deny"
print_stdout = "deny"
print_stderr = "deny"
await_holding_lock = "deny"
float_arithmetic = "warn"   # 金额必须用 Decimal；确需浮点处局部 allow 并注释原因
```

`clippy.toml` 设置 `allow-unwrap-in-tests = true`、`allow-expect-in-tests = true`；用 `disallowed-methods` 禁止 `chrono::Utc::now`、`std::time::SystemTime::now`，领域层再加 `std::time::Instant::now`。只在提供时钟的那一处 `#[allow(clippy::disallowed_methods)]` 并注释原因；启动阶段允许的 `expect` 同样局部 allow。clippy 只读取离 crate 最近的一个 `clippy.toml`，不会合并，所以子目录的配置要重复根目录的设置。

## 语言级自查

以下规则 clippy 和测试拦不住，提交前逐项确认：

- 金额构造与舍入符合 `rust_lang_CLAUDE.md`：用 `rust_decimal::Decimal`，显式 `round_dp_with_strategy`，没有 `as` 截断。
- 事务内没有远程调用：持有事务对象时没有 `.await` 第三方 HTTP、MQ 同步发送。
- 锁内没有 IO、`.await` 或可能重入的回调。
- 日志和错误信息中没有密钥、token、密码。
- 有界通道满时的策略已写明并处理 `Full`；不能丢的数据只会拒绝受理或熔断。
- 回调类型没有既实现下层回调 trait、又持有该下层客户端；`Arc` 持有关系不成环。
