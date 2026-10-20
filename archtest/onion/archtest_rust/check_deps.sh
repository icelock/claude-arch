#!/usr/bin/env bash
# 洋葱架构依赖检查，由人工定期在 workspace 根目录运行，输出为空才算通过（假设 crate 名与目录名一致）。
# 依赖方向由 Cargo.toml 保证，本脚本防止有人改错 Cargo.toml。
fail=0
deps() { cargo tree -p "$1" -e normal --prefix none ${2:+--depth $2} | tail -n +2; }
# 领域层：整棵依赖树中不得出现项目 crate、异步运行时和 IO crate
deps domain | grep -E '^(application|adapter-|app |tokio|sqlx|reqwest|redis|rdkafka|hyper)' && fail=1
# 领域层：直接依赖中不得有 serde。只查直接依赖，因为 rust_decimal、chrono 等可能通过 feature 间接带入 serde，那不算违规
deps domain 1 | grep -E '^serde ' && fail=1
# 应用层：直接依赖中项目 crate 只能是 domain，不得有 IO crate；tokio 只能启用无 IO 的 feature
deps application 1 | grep -E '^(adapter-|app |sqlx|reqwest|redis|rdkafka|hyper)' && fail=1
cargo tree -p application -e normal,features -i tokio --prefix none | grep -E 'tokio feature "(net|fs|process|io-util|rt)' && fail=1
# 适配器之间互不依赖
for c in $(ls crates | grep '^adapter-'); do
  deps "$c" 1 | grep -E '^adapter-' && { echo "$c 依赖了其他适配器"; fail=1; }
done
exit $fail
