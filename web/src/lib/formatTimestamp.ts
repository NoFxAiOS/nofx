/**
 * Format a timestamp for display in UI cards and tooltips.
 *
 * Rules:
 * - If timestamp is today: show only time (HH:MM)
 * - If timestamp is not today: show date and time (MM/DD HH:MM)
 * - Format according to user's language preference (zh-CN or en-US)
 *
 * @param timestamp - ISO 8601 timestamp string
 * @param language - User's language preference ('zh' or 'en')
 * @returns Formatted timestamp string
 *
 * @example
 * // Today at 14:30
 * formatTimestamp('2025-11-16T14:30:00Z', 'zh') // => "14:30"
 *
 * // Yesterday at 14:30
 * formatTimestamp('2025-11-15T14:30:00Z', 'zh') // => "11/15 14:30"
 */
export function formatTimestamp(timestamp: string, language: string): string {
  try {
    const date = new Date(timestamp)
    const now = new Date()

    // Check if the timestamp is today by comparing date strings
    const isToday = date.toDateString() === now.toDateString()

    // Determine locale based on language
    const locale = language === 'zh' ? 'zh-CN' : 'en-US'

    if (isToday) {
      // Today: show only time
      return date.toLocaleTimeString(locale, {
        hour: '2-digit',
        minute: '2-digit',
        hour12: false, // Use 24-hour format
      })
    } else {
      // Not today: show date and time
      return date.toLocaleString(locale, {
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        hour12: false, // Use 24-hour format
      })
    }
  } catch (error) {
    // Gracefully handle invalid timestamps
    console.error('Invalid timestamp:', timestamp, error)
    return '--:--'
  }
}
