import { useState, useEffect } from 'react'

interface GitHubStats {
  stars: number
  forks: number
  createdAt: string
  daysOld: number
  isLoading: boolean
  error: string | null
}

export function useGitHubStats(owner: string, repo: string): GitHubStats {
  const [stats, setStats] = useState<GitHubStats>({
    stars: 0,
    forks: 0,
    createdAt: '',
    daysOld: 0,
    isLoading: true,
    error: null,
  })

  useEffect(() => {
    const fetchGitHubStats = async () => {
      try {
        const response = await fetch(
          `https://api.github.com/repos/${owner}/${repo}`
        )

        if (!response.ok) {
          throw new Error('Failed to fetch GitHub stats')
        }

        const data = await response.json()

        // Calculate days since creation
        const createdDate = new Date(data.created_at)
        const now = new Date()
        const diffTime = Math.abs(now.getTime() - createdDate.getTime())
        const diffDays = Math.ceil(diffTime / (1000 * 60 * 60 * 24))

        setStats({
          stars: data.stargazers_count,
          forks: data.forks_count,
          createdAt: data.created_at,
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
