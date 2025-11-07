# fix(ui): resolve selectedExchange content overflow blocking buttons

## 📋 Type of Change

- [x] Bug fix (non-breaking change which fixes an issue)
- [ ] New feature (non-breaking change which adds functionality)
- [ ] Breaking change (fix or feature that would cause existing functionality to not work as expected)
- [ ] Documentation update
- [ ] Code refactoring
- [ ] Performance improvement
- [ ] Test update

## 🐛 Problem

When the `selectedExchange` content in the Exchange Configuration Modal was too long, it couldn't scroll and blocked the Cancel and Submit buttons at the bottom, making it impossible to interact with them.

## 🔧 Solution

Restructured the modal layout using flexbox to enable proper scrolling:

1. **Modal Container**: Added `flex flex-col max-h-[90vh]` to limit modal height and enable flex layout
2. **Header Section**: Made header non-shrinkable with `flex-shrink-0` to keep it always visible
3. **Content Area**: Wrapped form content in a scrollable container with `overflow-y-auto flex-1` to allow vertical scrolling
4. **Button Section**: Moved buttons outside scrollable area with `flex-shrink-0` and added top border for visual separation

## 📝 Changes Made

- Modified modal container to use flexbox layout with max height constraint
- Added scrollable content area for form fields
- Fixed button section to remain visible at bottom
- Improved visual separation with border between content and buttons

## ✅ Testing Steps

1. Open the AI Traders page
2. Click "Add Exchange" or "Edit Exchange" button
3. Select an exchange with long content (e.g., Binance with expanded guide)
4. Verify that:
   - Content area scrolls when content exceeds viewport
   - Cancel and Submit buttons remain visible and accessible at all times
   - Modal doesn't exceed 90% of viewport height
   - All form fields are accessible via scrolling

## 🖼️ Screenshots

### Before
- Content overflowed and buttons were blocked
- No scrolling capability

### After
- Content scrolls properly
- Buttons always visible at bottom
- Better user experience

## 🔗 Related Issues

N/A

## ✅ Checklist

- [x] Code compiles successfully (`npm run build`)
- [x] No linting errors
- [x] All tests pass (if applicable)
- [x] Documentation updated (if needed)
- [x] Commits follow conventional commits format
- [x] Branch is rebased on latest dev (if applicable)

## 📊 PR Size

- **Lines Changed**: ~10 lines
- **Files Changed**: 1 file
- **Size Category**: ✅ Small PR (< 300 lines)

## 💡 Additional Notes

This fix improves the user experience when configuring exchanges, especially for exchanges with extensive configuration options or long descriptions. The modal now handles content overflow gracefully while maintaining accessibility to all action buttons.

