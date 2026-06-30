/**
 * Text utilities
 *
 * stripLeadingIcons: removes decorative emoji/symbols at the start of a
 * translated string or title, so components can place their own icon without
 * showing a duplicate.
 */

/**
 * Strip leading decorative emoji/symbols and any following separators (spaces/colons/dots, etc.).
 */
export function stripLeadingIcons(input: string | undefined | null): string {
  if (!input) return ''
  let s = String(input)

  // 1) Strip common emoji/symbol blocks (arrows, misc symbols, geometric shapes, emoticons, etc.)
  //    Covers the common ranges; more compatible than using Unicode property classes.
  s = s.replace(
    /^[\s\u2190-\u21FF\u2300-\u23FF\u2460-\u24FF\u25A0-\u25FF\u2600-\u27BF\u2B00-\u2BFF\u1F000-\u1FAFF]+/u,
    ''
  )

  // 2) Strip any leading separators that may remain (spaces, hyphens, colons, middle dots, etc.)
  s = s.replace(/^[\s\-:•·]+/, '')

  return s.trim()
}

export default { stripLeadingIcons }
