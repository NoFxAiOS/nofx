import { useState, useEffect } from 'react'

interface GitHubStats {
  stars: number
  forks: number
  contributors: number
  createdAt: string
  daysOld: number
  isLoading: boolean
  error: string | null
}

export function useGitHubStats(owner: string, repo: string): GitHubStats {
  const [stats, setStats] = useState<GitHubStats>({
    stars: 0,
    forks: 0,
    contributors: 0,
    createdAt: '',
    daysOld: 0,
    isLoading: true,
    error: null,
  })

  useEffect(() => {
    const fetchGitHubStats = async () => {
      try {
        // Fetch basic repo info
        const repoRes = await fetch(`https://api.github.com/repos/${owner}/${repo}`)
        if (!repoRes.ok) throw new Error('Failed to fetch GitHub stats')
        const repoData = await repoRes.json()

        // Fetch contributors count (using Link header trick for large numbers, or length for small)
        // Since we can't easily parse Link header in client-side without exposing logic, 
        // we'll try a rough count or just a list length valid for first page (max 30 or 100).
        // For a more accurate count without pagination, we often check the 'Link' header of:
        // https://api.github.com/repos/{owner}/{repo}/contributors?per_page=1&anon=true
        let contributorsCount = 0
        try {
          const contribRes = await fetch(`https://api.github.com/repos/${owner}/${repo}/contributors?per_page=1&anon=true`)
          const linkHeader = contribRes.headers.get('Link')
          if (linkHeader) {
            const match = linkHeader.match(/page=(\d+)>; rel="last"/)
            if (match) {
              contributorsCount = parseInt(match[1])
            }
          }
          // If no link header, it means 1 page.
          if (contributorsCount === 0 && contribRes.ok) {
            // Fetch list to count (default page size 30)
            // actually per_page=1 returns 1. 
            // We should fetch with per_page=100 to get exact count if <100.
            const listRes = await fetch(`https://api.github.com/repos/${owner}/${repo}/contributors?per_page=100&anon=true`)
            if (listRes.ok) {
              const list = await listRes.json()
              contributorsCount = list.length
            }
          }
        } catch (e) {
          console.warn('Failed to fetch contributors:', e)
        }

        // Calculate days since creation
        const createdDate = new Date(repoData.created_at)
        const now = new Date()
        const diffTime = Math.abs(now.getTime() - createdDate.getTime())
        const diffDays = Math.ceil(diffTime / (1000 * 60 * 60 * 24))

        setStats({
          stars: repoData.stargazers_count,
          forks: repoData.forks_count,
          contributors: contributorsCount > 0 ? contributorsCount : 0, // Fallback
          createdAt: repoData.created_at,
          daysOld: diffDays,
          isLoading: false,
          error: null,
        })
      } catch (error) {
        console.error('Error fetching GitHub stats:', error)
        setStats((prev) => ({
          ...prev,
          isLoading: false,
          error: error instanceof Error ? error.message : 'Unknown error',
        }))
      }
    }

    fetchGitHubStats()
  }, [owner, repo])

  return stats
}
