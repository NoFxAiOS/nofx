import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import HeaderBar from './HeaderBar'

describe('diagnostic pages live under settings', () => {
  it('does not list system quality / research pages in the top nav', () => {
    render(
      <MemoryRouter>
        <HeaderBar language="zh" isLoggedIn />
      </MemoryRouter>
    )
    for (const name of ['系统质量', '因子挖掘', '晋升报告', '研究记录']) {
      expect(screen.queryByRole('button', { name })).toBeNull()
    }
  })
})
