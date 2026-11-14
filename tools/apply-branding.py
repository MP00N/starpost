#!/usr/bin/env python3
"""把站点名称、Logo、页脚写进 options 表。

为什么要单独做：项目 AGENTS.md 把源码里的品牌字符串（common/constants.go 的
SystemName、Footer 等）列为受保护内容，不能改。但后台「系统设置 → 站点」本来
就能覆盖这些值——那些值就存在 options 表里。所以安装时直接写库，等价于管理员
手动在后台改一遍，且不改任何受保护的源码。

用法：
    python3 tools/apply-branding.py data/one-api.db [/starpost-mark.svg]

退出码：
    0   没有变化，无需重启
    10  写入了新值，需要重启进程
    2   数据库不可用（不阻塞启动）

为什么用退出码而不是打印标记：/api/status 读的是启动时载入内存的选项，写完不
重启接口还是旧名字。之前用「打印 BRAND_CHANGED=」传递，一改文案就静默失效了。

注意：主题预设（starpost）不在这里。它由前端 cookie `theme_preset` 加
localStorage 决定，默认值写在 web/src/lib/theme-customization.ts 的
DEFAULT_THEME_CUSTOMIZATION。曾试图写 `theme.frontend`，那是已废弃的旧键，
会被 model/frontend_option_migration.go 强制归一为 default。
"""

from __future__ import annotations

import sqlite3
import sys

BRANDING = {
    "SystemName": "星驿 StarPost",
    "Footer": "星驿 StarPost · 驿路千里，一站通行",
}


def main() -> int:
    if len(sys.argv) < 2:
        print("usage: apply-branding.py <db> [logo-path]", file=sys.stderr)
        return 2

    db = sys.argv[1]
    logo = sys.argv[2] if len(sys.argv) > 2 else ""

    values = dict(BRANDING)
    if logo:
        values["Logo"] = logo

    try:
        con = sqlite3.connect(db)
    except sqlite3.Error as exc:
        print(f"branding skipped: {exc}", file=sys.stderr)
        return 2

    try:
        changed = 0
        for key, value in values.items():
            row = con.execute("SELECT value FROM options WHERE key = ?", (key,)).fetchone()
            if row and row[0] == value:
                continue
            con.execute(
                "INSERT INTO options (key, value) VALUES (?, ?) "
                "ON CONFLICT(key) DO UPDATE SET value = excluded.value",
                (key, value),
            )
            changed += 1
        con.commit()
        print(f"branding: {values['SystemName']} · {changed} changed")
        return 10 if changed else 0
    except sqlite3.Error as exc:
        print(f"branding skipped: {exc}", file=sys.stderr)
        return 2
    finally:
        con.close()


if __name__ == "__main__":
    raise SystemExit(main())
