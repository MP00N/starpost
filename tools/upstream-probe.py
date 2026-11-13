#!/usr/bin/env python3
"""对接上游前，先把它和本地价格对一遍。

用途：给一个上游的 base_url + key，脚本会尽量读到上游的模型清单和价格倍率，
再和本地的 /api/pricing 逐条比对，告诉你：

  · 上游有、本地没定价的模型（漏配，用户能调但会按 0 计价）
  · 两边都有的模型，倍率差多少（差得多说明你卖亏了或卖贵了）
  · 本地有、上游没开的模型（不用配）
  · 缓存读写倍率的差异（群里说「缓存读写单独拆开算」，这就是那一项）

用法：
    python3 tools/upstream-probe.py --upstream https://api.example.com --key sk-xxx
    python3 tools/upstream-probe.py --upstream ... --local http://127.0.0.1:3000
    python3 tools/upstream-probe.py --upstream ... --format json > diff.json

前提说明（重要）：
    倍率只有在上游本身是 new-api / one-api 系的时候才读得到，因为它会暴露
    /api/pricing。其他上游只给 /v1/models（一串模型名），没有价格，脚本会
    如实说明「拿不到倍率」，不会编一个数字出来。
"""

from __future__ import annotations

import argparse
import json
import sys
import urllib.error
import urllib.request

TIMEOUT = 15

CACHE_KEYS = (
    "cache_ratio",
    "create_cache_ratio",
    "cache_read_ratio",
    "cache_write_ratio",
)


def http_json(url: str, key: str | None = None) -> tuple[int, dict | list | None, str]:
    req = urllib.request.Request(url, headers={"Accept": "application/json"})
    if key:
        req.add_header("Authorization", f"Bearer {key}")
    try:
        with urllib.request.urlopen(req, timeout=TIMEOUT) as resp:
            body = resp.read().decode("utf-8", "replace")
            try:
                return resp.status, json.loads(body), ""
            except json.JSONDecodeError:
                return resp.status, None, "响应不是 JSON（可能被风控或跳转到登录页）"
    except urllib.error.HTTPError as exc:
        return exc.code, None, f"HTTP {exc.code}"
    except Exception as exc:  # noqa: BLE001 - 网络错误种类多，统一报告即可
        return 0, None, str(exc)


def load_pricing(base: str, key: str | None) -> tuple[dict[str, dict], str]:
    """读一个 new-api 系的 /api/pricing。

    返回 (模型表, 诊断)。诊断区分三种情况，因为它们的处置完全不同：
      ""        读到了，且有模型
      "empty"   读到了，但一个模型都没有（站点还没配渠道）
      "unreach" 读不到（地址错、鉴权失败、不是 new-api 系）
    """
    status, payload, err = http_json(f"{base.rstrip('/')}/api/pricing", key)
    if status != 200 or not isinstance(payload, dict):
        return {}, "unreach"
    data = payload.get("data")
    out: dict[str, dict] = {}
    if isinstance(data, list):
        for item in data:
            if not isinstance(item, dict):
                continue
            name = item.get("model_name") or item.get("name")
            if not name:
                continue
            out[name] = {
                "ratio": item.get("model_ratio", item.get("quota_type")),
                "completion": item.get("completion_ratio"),
                **{k: item.get(k) for k in CACHE_KEYS if item.get(k) is not None},
            }
    elif isinstance(data, dict):
        for name, item in data.items():
            if not isinstance(item, dict):
                continue
            out[name] = {
                "ratio": item.get("model_ratio"),
                "completion": item.get("completion_ratio"),
                **{k: item.get(k) for k in CACHE_KEYS if item.get(k) is not None},
            }
    return out, ("" if out else "empty")


def load_model_ids(base: str, key: str | None) -> list[str]:
    """兜底：OpenAI 兼容的 /v1/models，只有 id，没有价格。"""
    status, payload, _ = http_json(f"{base.rstrip('/')}/v1/models", key)
    if status != 200 or not isinstance(payload, dict):
        return []
    items = payload.get("data")
    if not isinstance(items, list):
        return []
    return [i["id"] for i in items if isinstance(i, dict) and i.get("id")]


def fmt(value: object) -> str:
    if value is None:
        return "—"
    if isinstance(value, float):
        return f"{value:.3f}".rstrip("0").rstrip(".")
    return str(value)


def main() -> int:
    ap = argparse.ArgumentParser(description="对比上游与本地模型倍率")
    ap.add_argument("--upstream", required=True, help="上游 base_url")
    ap.add_argument("--key", default=None, help="上游 API Key（需要鉴权时填）")
    ap.add_argument("--local", default="http://127.0.0.1:3000", help="本地 base_url")
    ap.add_argument("--format", choices=["table", "json"], default="table")
    args = ap.parse_args()

    upstream, up_diag = load_pricing(args.upstream, args.key)
    local, _local_diag = load_pricing(args.local, None)

    if up_diag == "empty":
        print("上游的 /api/pricing 能打开，但里面没有任何模型——通常是对方还没配渠道，"
              "或者这个地址不是 new-api 系。没有价格，就没法比倍率。", file=sys.stderr)
        return 0

    if not upstream:
        ids = load_model_ids(args.upstream, args.key)
        if not ids:
            print("✗ 上游既没有 /api/pricing，也没有 /v1/models —— 地址或 Key 不对，"
                  "或者接口不是 OpenAI 兼容的。", file=sys.stderr)
            return 2
        print(f"上游不是 new-api 系：只拿到 {len(ids)} 个模型名，没有价格。")
        print("这种情况下无法给出倍率建议——倍率必须由你自己按上游账单折算。\n")
        missing = [m for m in ids if m not in local]
        if missing:
            print(f"本地没有定价的模型（{len(missing)} 个），上线前要补：")
            for m in sorted(missing):
                print(f"  · {m}")
        return 0

    rows = []
    for model, up in sorted(upstream.items()):
        lo = local.get(model)
        if lo is None:
            rows.append((model, up.get("ratio"), None, up.get("completion"), None, "本地缺价"))
            continue
        try:
            delta = float(up["ratio"]) - float(lo["ratio"])
        except (TypeError, ValueError, KeyError):
            delta = None
        rows.append((model, up.get("ratio"), lo.get("ratio"), up.get("completion"), delta, ""))

    if args.format == "json":
        print(json.dumps(
            {"upstream": upstream, "local": local,
             "only_upstream": [r[0] for r in rows if r[5]],}, ensure_ascii=False, indent=2))
        return 0

    print(f"上游 {args.upstream}")
    print(f"本地 {args.local}")
    print(f"上游模型 {len(upstream)} 个，本地 {len(local)} 个\n")

    print(f"{'模型':32} {'上游倍率':>9} {'本地倍率':>9} {'差值':>9} {'缓存读':>8}  备注")
    print("-" * 88)
    for model, up_r, lo_r, comp, delta, note in rows:
        up_cache = fmt(upstream[model].get("cache_ratio") or upstream[model].get("cache_read_ratio"))
        flag = note
        if not flag and delta is not None:
            if delta > 0.05:
                flag = "上游更贵，按本地价会亏"
            elif delta < -0.05:
                flag = "上游更便宜，有让利空间"
        print(f"{model[:32]:32} {fmt(up_r):>9} {fmt(lo_r):>9} {fmt(delta):>9} {up_cache:>8}  {flag}")

    only_local = [m for m in local if m not in upstream]
    if only_local:
        print(f"\n本地有、上游没开的模型：{len(only_local)} 个（不用配，列前 10 个）")
        for m in sorted(only_local)[:10]:
            print(f"  · {m}")

    print("\n倍率怎么用：")
    print("  · 上游倍率 < 本地倍率 → 你在这条线路上有毛利，保持即可")
    print("  · 上游倍率 > 本地倍率 → 按本地价卖会亏，要么调高本地倍率，要么只做分组")
    print("  · 缓存读写倍率单独看：读便宜、写贵，混在一起算会低估成本")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
