import { render, screen, waitFor } from '@testing-library/react';
import { AuthProvider, useAuth } from '../AuthContext';
import { vi, describe, it, expect, beforeEach } from 'vitest';
import React from 'react';

// Mock fetch
global.fetch = vi.fn();

const TestComponent = () => {
  const { user } = useAuth();
  return <div>{user?.invite_code || 'No Code'}</div>;
};

describe('AuthContext Auto-Refresh', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
  });

  it('refreshes user profile on load if token exists', async () => {
    // Setup initial state in localStorage (stale data without invite_code)
    const staleUser = { id: 'user1', email: 'test@example.com' }; 
    localStorage.setItem('auth_token', 'valid-token');
    localStorage.setItem('auth_user', JSON.stringify(staleUser));

    // Mock API response with fresh data (including invite_code)
    (global.fetch as any).mockResolvedValue({
      ok: true,
      json: async () => ({
        id: 'user1',
        email: 'test@example.com',
        invite_code: 'NEWCODE123'
      }),
    });

    render(
      <AuthProvider>
        <TestComponent />
      </AuthProvider>
    );

    // Initial render might show "No Code" (from localStorage), but effect should trigger update
    // We wait for the API result to be reflected
    await waitFor(() => {
      expect(screen.getByText('NEWCODE123')).toBeInTheDocument();
    });

    // Check if localStorage was updated with the new data
    const updatedUser = JSON.parse(localStorage.getItem('auth_user') || '{}');
    expect(updatedUser.invite_code).toBe('NEWCODE123');
    
    // Verify API call
    expect(global.fetch).toHaveBeenCalledWith(
      expect.stringContaining('/user/me'),
      expect.objectContaining({
        headers: expect.objectContaining({
          'Authorization': 'Bearer valid-token'
        })
      })
    );
  });
});
