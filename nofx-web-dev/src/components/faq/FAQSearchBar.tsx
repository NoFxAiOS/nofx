import { Search, X } from 'lucide-react'

interface FAQSearchBarProps {
  searchTerm: string
  onSearchChange: (value: string) => void
  placeholder?: string
}

export function FAQSearchBar({
  searchTerm,
  onSearchChange,
  placeholder = 'Search FAQ...',
}: FAQSearchBarProps) {
  return (
    <div className="relative">
      <Search
        className="absolute left-4 top-1/2 transform -translate-y-1/2 w-5 h-5"
        style={{ color: 'var(--text-secondary)' }}
      />
      <input
        type="text"
        value={searchTerm}
        onChange={(e) => onSearchChange(e.target.value)}
        placeholder={placeholder}
        className="w-full pl-12 pr-12 py-3 rounded-lg text-base transition-all focus:outline-none focus:ring-2"
        style={{
          background: '#FFFFFF',
          border: '1px solid #E5E7EB',
          color: 'var(--text-primary)',
        }}
        onFocus={(e) => {
          e.target.style.borderColor = 'var(--brand-red)'
          e.target.style.boxShadow = '0 0 0 3px rgba(229, 0, 18, 0.1)'
        }}
        onBlur={(e) => {
          e.target.style.borderColor = '#E5E7EB'
          e.target.style.boxShadow = 'none'
        }}
      />
      {searchTerm && (
        <button
          onClick={() => onSearchChange('')}
          className="absolute right-4 top-1/2 transform -translate-y-1/2 hover:opacity-70 transition-opacity"
          style={{ color: 'var(--text-secondary)' }}
        >
          <X className="w-5 h-5" />
        </button>
      )}
    </div>
  )
}
