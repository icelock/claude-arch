#!/usr/bin/env bash
# 单 crate 分层检查，由人工定期在项目根目录运行，输出为空才算通过。
#
# 多 crate 工程由 Cargo.toml 保证依赖方向，另外人工检查：
#   cargo tree -p <crate> -e normal --depth 1 中不应出现上层 crate；
#   service 的直接依赖中不应出现 sqlx、redis、rdkafka、reqwest。
#   只看直接依赖，因为 service 依赖 infra，传递依赖里必然有这些 crate。
#
# 层间方向：下层不得引用上层；跨层引用必须写成 crate::<层> 完整路径或 crate::{...} 分组
fail=0
grep_or_fail() { if grep -rnE "$2" $1; then echo "违反规则：$1 中出现 $2"; fail=1; fi; }
forbid() { grep_or_fail "$1" "crate::($2)\b|crate::\{[^}]*\b($2)\b"; }
forbid "src/common"  'entry|service|infra|bootstrap'
forbid "src/infra"   'entry|service|bootstrap'
forbid "src/service" 'entry|bootstrap'
forbid "src/entry"   'infra|bootstrap'
# 外部依赖：service、common 不直接用中间件/SDK
grep_or_fail "src/service src/common" '\b(sqlx|redis|rdkafka|reqwest|tokio_tungstenite)::|tokio::(net|fs)\b'
exit $fail
