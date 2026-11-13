#!/usr/bin/env python3
"""对账台：把用量日志里的 token、缓存、额度三者核到一起。

官方有的：用量日志列表（一条条看）。
官方没有的：把同一批日志按模型汇总成账，并且**反过来校验计费本身**。

为什么要验：一条日志里 quota 是结果，token 和倍率是依据。把依据重算一遍再和
结果比，就能抓出「计费悄悄算错」——这种 bug 不报错、不崩溃，只错钱。

用法：
    python3 tools/billing-audit.py                          # 看近 7 天
    python3 tools/billing-audit.py --days 1 --top 15
    python3 tools/billing-audit.py --db /path/one-api.db --json > audit.json
    python3 tools/billing-audit.py --tolerance 0.02         # 偏差阈值

说明：
    · 只读打开数据库，不会写入任何东西。
    · 缓存 token 不在列里，藏在 logs.other 这个 JSON 字段，这是对账最容易
      对不上的地方，脚本会显式指出来。
    · 「期望额度」按 one-api 的标准公式估算；如果站点用了表达式计费或模型
      定价（按次），这种行会单独归类，不会被误判成 bug。
"""

from __future__ import annotations

import argparse
import json
import os
import sqlite3
import sys
import time
from collections import defaultdict

LOG_TYPE_CONSUME = 2

# one-api: 1 美元 = QuotaPerUnit 个额度。项目里默认 500000。
DEFAULT_QUOTA_PER_UNIT = 500000


def connect_ro(path: str) -> sqlite3.Connection:
    if not os.path.exists(path):
        sys.exit(f"数据库不存在：{path}")
    # 只读 + 不创建：避免在正在跑的站点上留下 -wal 文件
    return sqlite3.connect(f"file:{path}?mode=ro", uri=True)


def parse_other(raw: str | None) -> dict:
    if not raw:
        return {}
    try:
        value = json.loads(raw)
        return value if isinstance(value, dict) else {}
    except json.JSONDecodeError:
        return {}


def as_float(source: dict, *keys: str) -> float | None:
    for key in keys:
        if key in source and source[key] is not None:
            try:
                return float(source[key])
            except (TypeError, ValueError):
                continue
    return None


def main() -> int:
    ap = argparse.ArgumentParser(description="用量对账与计费偏差检测")
    ap.add_argument("--db", default="data/one-api.db")
    ap.add_argument("--days", type=float, default=7)
    ap.add_argument("--top", type=int, default=20)
    ap.add_argument("--tolerance", type=float, default=0.03,
                    help="期望值与实际额度允许的相对偏差，默认 3%%")
    ap.add_argument("--json", action="store_true")
    args = ap.parse_args()

    con = connect_ro(args.db)
    since = int(time.time() - args.days * 86400)

    rows = con.execute(
        """
        SELECT model_name, prompt_tokens, completion_tokens, quota,
               channel_name, other
        FROM logs
        WHERE type = ? AND created_at >= ?
        """,
        (LOG_TYPE_CONSUME, since),
    ).fetchall()

    if not rows:
        print(f"近 {args.days:g} 天没有消费日志。先用 run.sh 起站点发几条请求再来。")
        return 0

    per_model = defaultdict(lambda: {
        "requests": 0, "prompt": 0, "completion": 0,
        "cache_read": 0, "cache_write": 0, "quota": 0,
        "expected": 0.0, "unpriced": 0,
    })

    anomalies = {
        "quota_zero_with_tokens": [],
        "cache_exceeds_prompt": [],
        "broken_other_json": [],
        "billing_drift": [],
        "negative_tokens": [],
        "no_ratio": [],
    }

    cache_saved_quota = 0.0
    price_mode_rows = 0

    for model, prompt, completion, quota, channel, other_raw in rows:
        other = parse_other(other_raw)
        if other_raw and not other:
            anomalies["broken_other_json"].append(
                {"model": model, "raw": (other_raw or "")[:80]}
            )

        cache_read = int(as_float(other, "cache_tokens", "cache_read_tokens") or 0)
        cache_write = int(as_float(other, "cache_creation_tokens", "cache_write_tokens") or 0)
        model_ratio = as_float(other, "model_ratio")
        completion_ratio = as_float(other, "completion_ratio")
        cache_ratio = as_float(other, "cache_ratio")
        group_ratio = as_float(other, "group_ratio") or 1.0
        model_price = as_float(other, "model_price")

        bucket = per_model[model]
        bucket["requests"] += 1
        bucket["prompt"] += prompt or 0
        bucket["completion"] += completion or 0
        bucket["cache_read"] += cache_read
        bucket["cache_write"] += cache_write
        bucket["quota"] += quota or 0

        if (prompt or 0) < 0 or (completion or 0) < 0:
            anomalies["negative_tokens"].append({"model": model})
        if (quota or 0) == 0 and ((prompt or 0) + (completion or 0)) > 0:
            anomalies["quota_zero_with_tokens"].append(
                {"model": model, "prompt": prompt, "completion": completion}
            )
        if cache_read > (prompt or 0):
            anomalies["cache_exceeds_prompt"].append(
                {"model": model, "cache_read": cache_read, "prompt": prompt}
            )

        # 按次定价 / 表达式计费：不能套用倍率公式，单独归类
        if model_price:
            price_mode_rows += 1
            continue
        if model_ratio is None or completion_ratio is None:
            anomalies["no_ratio"].append({"model": model})
            continue

        # one-api 标准公式：缓存读按 cache_ratio 打折，普通 prompt 扣掉缓存部分
        base = max((prompt or 0) - cache_read, 0)
        expected = (
            base * model_ratio
            + cache_read * (cache_ratio if cache_ratio is not None else 1.0) * model_ratio
            + (completion or 0) * completion_ratio * model_ratio
        ) * group_ratio
        bucket["expected"] += expected

        # 缓存省下来的钱：这部分 token 若按原价算会多花的额度
        if cache_ratio is not None and cache_ratio < 1 and cache_read:
            cache_saved_quota += cache_read * (1 - cache_ratio) * model_ratio * group_ratio

        actual = quota or 0
        if expected > 0:
            drift = abs(actual - expected) / expected
            if drift > args.tolerance:
                anomalies["billing_drift"].append(
                    {
                        "model": model,
                        "expected": round(expected, 2),
                        "actual": actual,
                        "drift": round(drift, 4),
                    }
                )

    total_quota = sum(b["quota"] for b in per_model.values())
    total_expected = sum(b["expected"] for b in per_model.values())

    if args.json:
        print(json.dumps(
            {
                "window_days": args.days,
                "rows": len(rows),
                "models": per_model,
                "anomalies": anomalies,
                "cache_saved_quota": round(cache_saved_quota, 2),
            },
            ensure_ascii=False, indent=2, default=str,
        ))
        return 0

    print(f"窗口：近 {args.days:g} 天 · 消费日志 {len(rows)} 条 · 模型 {len(per_model)} 个")
    print(f"实收额度 {total_quota:,} · 按倍率重算 {total_expected:,.0f} "
          f"· 差额 {total_quota - total_expected:+,.0f}")
    if price_mode_rows:
        print(f"（其中 {price_mode_rows} 条是按次/表达式计费，不参与倍率重算）")
    print()

    header = f"{'模型':30} {'请求':>6} {'缓存命中':>9} {'缓存读':>10} {'缓存写':>9} {'实收':>11} {'重算':>11}"
    print(header)
    print("-" * len(header))
    for model, b in sorted(per_model.items(), key=lambda kv: kv[1]["quota"], reverse=True)[: args.top]:
        denom = b["prompt"] + b["cache_read"]
        hit = (b["cache_read"] / denom * 100) if denom else 0.0
        print(f"{model[:30]:30} {b['requests']:>6} {hit:>8.1f}% "
              f"{b['cache_read']:>10,} {b['cache_write']:>9,} "
              f"{b['quota']:>11,} {b['expected']:>11,.0f}")

    print()
    if cache_saved_quota > 0:
        usd = cache_saved_quota / DEFAULT_QUOTA_PER_UNIT
        # 小额时 2 位小数会显示成 0.00，用「分」表示更有意义
        if usd < 0.01:
            money = f"{usd * 100:.2f} 美分"
        else:
            money = f"{usd:.2f} 美元"
        print(f"缓存帮你省了约 {cache_saved_quota:,.0f} 额度 ≈ {money}"
              f"（按 1 美元 = {DEFAULT_QUOTA_PER_UNIT:,} 额度）")
    else:
        print("这段时间没有产生缓存折扣，或缓存倍率配置为 1（不打折）。")

    labels = {
        "quota_zero_with_tokens": "有 token 但额度为 0（漏计费，或模型免费）",
        "cache_exceeds_prompt": "缓存读 token 大于 prompt（上游用量可疑）",
        "broken_other_json": "other 字段不是合法 JSON（无法对账）",
        "billing_drift": "实收与按倍率重算差太多（计费偏差）",
        "negative_tokens": "token 数为负",
        "no_ratio": "日志里没有倍率字段（按次计费或旧版本日志）",
    }
    fired = [(k, v) for k, v in anomalies.items() if v]
    print()
    if not fired:
        print("✓ 没发现计费异常。")
        return 0

    print(f"发现 {sum(len(v) for _, v in fired)} 处可疑：")
    for key, items in fired:
        print(f"\n  {labels[key]} —— {len(items)} 条")
        for item in items[:5]:
            detail = ", ".join(f"{k}={v}" for k, v in item.items())
            print(f"    · {detail}")
        if len(items) > 5:
            print(f"    … 其余 {len(items) - 5} 条省略")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
