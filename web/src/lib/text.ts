/**
 * 文本工具
 *
 * stripLeadingIcons: 去掉翻译文案或标题前面用于装饰的 Emoji/符号，
 * 以便在组件里自行放置图标时不重复显示。
 */

/**
 * 去掉开头的装饰性 Emoji/符号以及随后的分隔符（空格/冒号/点号等）。
 */
export function stripLeadingIcons(input: string | undefined | null): string {
  if (!input) return ''
  let s = String(input)

  // 1) 去除常见的 Emoji/符号块（箭头、杂项符号、几何图形、表情等）
  //    覆盖常见范围，兼容性好于使用 Unicode 属性类。
  s = s.replace(
    /^[\s\u2190-\u21FF\u2300-\u23FF\u2460-\u24FF\u25A0-\u25FF\u2600-\u27BF\u2B00-\u2BFF\u1F000-\u1FAFF]+/u,
    ''
  )

  // 2) 去掉开头可能残留的分隔符（空格、连字符、冒号、居中点等）
  s = s.replace(/^[\s\-:•·]+/, '')

  return s.trim()
}

export default { stripLeadingIcons }
