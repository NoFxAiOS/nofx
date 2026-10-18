import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import ResearchLogPage from './ResearchLogPage'
import { RESEARCH_LOG } from '../constants/researchLog'

describe('ResearchLogPage', () => {
  it('renders every entry and filters by verdict', () => {
    render(<ResearchLogPage language="zh" />)
    expect(screen.getAllByRole('heading', { level: 2 })).toHaveLength(
      RESEARCH_LOG.length
    )
    fireEvent.click(screen.getByRole('button', { name: '进行中' }))
    const running = RESEARCH_LOG.filter((e) => e.verdict === 'running')
    expect(screen.getAllByRole('heading', { level: 2 })).toHaveLength(
      running.length
    )
  })
})
