#!/usr/bin/env python3
"""Vue/TS 项目批量注释后的体检扫描。

检查 6 类常见的 Python 脚本批量处理污染：
1. Python 关键字残留 (None/True/False/pass)
2. JSDoc 插在 import {} 中间
3. HTML 注释插在标签属性中间
4. :class / :style 对象尾逗号
5. script 区块泄漏 HTML 标签
6. style 区块泄漏 JS 代码
"""

import os
import re
import sys


def scan_region(content: str):
    """将 .vue 文件切分为 template/script/style 三个区块。"""
    regions = {'template': [], 'script': [], 'style': []}
    current = None
    for i, line in enumerate(content.splitlines(), 1):
        if re.match(r'^\s*<template[\s>]', line):
            current = 'template'
            continue
        if '</template>' in line:
            current = None
            continue
        if re.match(r'^\s*<script[\s>]', line):
            current = 'script'
            continue
        if '</script>' in line:
            current = None
            continue
        if re.match(r'^\s*<style[\s>]', line):
            current = 'style'
            continue
        if '</style>' in line:
            current = None
            continue
        if current:
            regions[current].append((i, line))
    return regions


def run_checks(root: str):
    issues = []
    file_count = 0

    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = [d for d in dirnames if d != 'node_modules']
        for fname in filenames:
            if not fname.endswith(('.vue', '.ts')) or fname.endswith('.test.ts'):
                continue
            fpath = os.path.join(dirpath, fname)
            file_count += 1
            try:
                with open(fpath, encoding='utf-8') as f:
                    content = f.read()
                    lines = content.splitlines()
            except Exception as e:
                issues.append(('读取失败', fpath, 0, str(e)))
                continue

            regions = scan_region(content)

            # 检查 1: Python 关键字残留（script 区或 ts 文件中孤立出现）
            region_lines = []
            if fname.endswith('.ts'):
                region_lines = [(i + 1, l) for i, l in enumerate(lines)]
            else:
                region_lines = regions['script']
            for ln, line in region_lines:
                s = line.strip()
                if s in ('None', 'True', 'False', 'pass'):
                    issues.append(('Python关键字残留', fpath, ln, s))

            # 检查 2: JSDoc 插在 import {} 中间
            # 检测多行 import { ... } 块内部包含 /** ... */
            pattern = r'import\s*\{([^}]*)\}\s*from\s*["\']'
            for m in re.finditer(pattern, content, re.DOTALL):
                inner = m.group(1)
                if '/**' in inner or '/*' in inner:
                    ln = content[:m.start()].count('\n') + 1
                    issues.append(('JSDoc在import{}中间', fpath, ln, inner[:80].strip()))

            # 检查 3: HTML 注释在模板的开始标签属性中间
            # 扫描 template 区块跨行的开始标签：从 <tag 开始到 > 之间如果有 <!-- 就是问题
            in_tag = False
            tag_start = 0
            tag_buf = []
            for ln, line in regions['template']:
                stripped = line.strip()
                if not in_tag:
                    if re.match(r'^<\w+', stripped) and not stripped.startswith('</'):
                        in_tag = True
                        tag_start = ln
                        tag_buf = [stripped]
                        if stripped.endswith('>') or stripped.endswith('/>'):
                            in_tag = False
                            tag_buf = []
                else:
                    tag_buf.append(stripped)
                    if stripped.endswith('>') or stripped.endswith('/>'):
                        in_tag = False
                        full = ' '.join(tag_buf)
                        if '<!--' in full:
                            issues.append(('HTML注释在标签属性中', fpath, tag_start, full[:100]))
                        tag_buf = []

            # 检查 4: :class / :style 对象尾逗号
            for ln, line in regions['template'] + regions['script']:
                if re.search(r":class=\"\{[^}]+\,\s*\}\"", line) or \
                   re.search(r":style=\"\{[^}]+\,\s*\}\"", line):
                    issues.append(('尾逗号 :class/:style', fpath, ln, line.strip()[:80]))

            # 检查 5: script 区块泄漏 HTML 标签
            for ln, line in regions['script']:
                s = line.strip()
                if s.startswith('<!--') and '-->' not in s:
                    issues.append(('script中未闭合HTML注释', fpath, ln, s[:60]))
                if (re.match(r'^<\w+', s) or s.startswith('</')) and \
                   not s.startswith('<!--') and not s.startswith('*'):
                    issues.append(('script中有HTML标签', fpath, ln, s[:60]))

            # 检查 6: style 区块泄漏 JS 代码
            for ln, line in regions['style']:
                s = line.strip()
                if s.startswith('import ') and 'from ' in s:
                    issues.append(('style中有JS import', fpath, ln, s[:60]))
                if s.startswith(('function ', 'const ', 'let ', 'var ')):
                    issues.append(('style中有JS代码', fpath, ln, s[:60]))

            # 结构完整性
            open_t = content.count('<template')
            close_t = content.count('</template>')
            open_s = content.count('<script')
            close_s = content.count('</script>')
            open_st = content.count('<style')
            close_st = content.count('</style>')
            if open_t != close_t:
                issues.append(('template标签不匹配', fpath, 0,
                               f'{open_t}开 {close_t}闭'))
            if open_s != close_s:
                issues.append(('script标签不匹配', fpath, 0,
                               f'{open_s}开 {close_s}闭'))
            if open_st != close_st:
                issues.append(('style标签不匹配', fpath, 0,
                               f'{open_st}开 {close_st}闭'))

    # 输出结果
    print('=' * 60)
    print(f'体检扫描: {file_count} 个文件')
    print(f'发现问题: {len(issues)} 处')
    print('=' * 60)

    if issues:
        sev_order = ['Python关键字残留', 'JSDoc在import{}中间',
                     'HTML注释在标签属性中', 'script中有HTML标签',
                     'script中未闭合HTML注释', 'style中有JS import',
                     'style中有JS代码', 'template标签不匹配',
                     'script标签不匹配', 'style标签不匹配',
                     '尾逗号 :class/:style', '读取失败']
        issues.sort(key=lambda x: sev_order.index(x[0]) if x[0] in sev_order else 999)
        for sev, fp, ln, detail in issues:
            loc = f':{ln}' if ln > 0 else ''
            print(f'  [{sev}] {fp}{loc}')
            print(f'    {detail}')
    else:
        print('  ✅ 无任何结构性污染问题')


if __name__ == '__main__':
    target = sys.argv[1] if len(sys.argv) > 1 else '.'
    run_checks(os.path.abspath(target))
