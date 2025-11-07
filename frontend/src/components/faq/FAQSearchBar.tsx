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
        style={{ color: '#848E9C' }}
      />
      <input
        type="text"
        value={searchTerm}
        onChange={(e) => onSearchChange(e.target.value)}
        placeholder={placeholder}
        className="w-full pl-12 pr-12 py-3 rounded-lg text-base transition-all focus:outline-none focus:ring-2"
        style={{
          background: '#1E2329',
          border: '1px solid #2B3139',
          color: '#EAECEF',
        }}
        onFocus={(e) => {
          e.target.style.borderColor = '#F0B90B'
          e.target.style.boxShadow = '0 0 0 3px rgba(240, 185, 11, 0.1)'
        }}
        onBlur={(e) => {
          e.target.style.borderColor = '#2B3139'
          e.target.style.boxShadow = 'none'
        }}
      />
      {searchTerm && (
        <button
          onClick={() => onSearchChange('')}
          className="absolute right-4 top-1/2 transform -translate-y-1/2 hover:opacity-70 transition-opacity"
          style={{ color: '#848E9C' }}
        >
          <X className="w-5 h-5" />
        </button>
      )}
    </div>
  )
}
