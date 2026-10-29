#!/usr/bin/env python3
"""统计 Vue/TS 项目的注释覆盖率。

SonarQube 统计口径：
- 独立注释行（//、/*、*/、*、<!--、-->）
- 块注释中间行计入
- 排除测试文件
"""

import os
import sys


def count_comment_lines(content: str) -> int:
    """统计文件中的注释行数量（块注释中间行计入）。"""
    comment_lines = 0
    in_block_comment = False
    for stripped in (l.strip() for l in content.splitlines()):
        if in_block_comment:
            comment_lines += 1
            if '*/' in stripped:
                in_block_comment = False
            continue

        if stripped.startswith('//'):
            comment_lines += 1
        elif stripped.startswith('<!--') or stripped.startswith('-->'):
            comment_lines += 1
        elif stripped.startswith('*') and not stripped.startswith('*/'):
            comment_lines += 1
        elif stripped.startswith('/*'):
            comment_lines += 1
            if '*/' not in stripped:
                in_block_comment = True
    return comment_lines


def scan(root: str):
    total_lines = 0
    total_comment = 0
    file_stats = []

    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = [d for d in dirnames if d != 'node_modules']
        for fname in filenames:
            if not fname.endswith(('.vue', '.ts')) or fname.endswith('.test.ts'):
                continue
            fpath = os.path.join(dirpath, fname)
            try:
                with open(fpath, encoding='utf-8') as f:
                    content = f.read()
            except Exception:
                continue

            tl = content.count('\n') + (0 if content.endswith('\n') or content == '' else 1)
            cl = count_comment_lines(content)
            pct = cl / tl * 100 if tl else 0
            file_stats.append((pct, cl, tl, os.path.relpath(fpath, root)))
            total_lines += tl
            total_comment += cl

    file_stats.sort()
    overall = total_comment / total_lines * 100 if total_lines else 0

    print('=' * 60)
    print(f'总代码行数: {total_lines}')
    print(f'总注释行数: {total_comment}')
    print(f'整体注释率: {overall:.2f}%')
    target = total_lines * 0.15
    gap = int(target - total_comment)
    if gap > 0:
        print(f'距离 15% 目标还差约 {gap} 行注释')
    else:
        print(f'已达到 15%！超出 {-gap} 行')
    print('=' * 60)

    below = [(p, c, t, fp) for p, c, t, fp in file_stats if p < 15]
    print(f'\n低于 15% 的文件: {len(below)}')
    if below:
        below.sort(key=lambda x: x[2] * 0.15 - x[1], reverse=True)
        total_needed = sum(max(0, int(t * 0.15) - c) + 2 for _, c, t, _ in below)
        print(f'这些文件总共还需要约 {total_needed} 行注释')
        print('\n按缺口大小排序 TOP 10:')
        for pct, cl, tl, fp in below[:10]:
            needed = max(0, int(tl * 0.15) - cl) + 2
            print(f'  {pct:5.1f}%  {cl:4d}/{tl:4d}  需+{needed:3d}行  {fp}')


if __name__ == '__main__':
    target = sys.argv[1] if len(sys.argv) > 1 else '.'
    scan(os.path.abspath(target))
