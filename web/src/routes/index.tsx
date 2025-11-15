import { createBrowserRouter, Navigate } from 'react-router-dom'
import MainLayout from '../layouts/MainLayout'
import AuthLayout from '../layouts/AuthLayout'
import { LandingPage } from '../pages/LandingPage'
import { FAQPage } from '../pages/FAQPage'
import { LoginPage } from '../components/LoginPage'
import { RegisterPage } from '../components/RegisterPage'
import { ResetPasswordPage } from '../components/ResetPasswordPage'
import { CompetitionPage } from '../components/CompetitionPage'
import { AITradersPage } from '../pages/AITradersPage'
import TraderDashboard from '../pages/TraderDashboard'

export const router = createBrowserRouter([
  {
    path: '/',
    element: <LandingPage />,
  },
  // Auth routes - using AuthLayout
  {
    element: <AuthLayout />,
    children: [
      {
        path: '/login',
        element: <LoginPage />,
      },
      {
        path: '/register',
        element: <RegisterPage />,
      },
      {
        path: '/reset-password',
        element: <ResetPasswordPage />,
      },
    ],
  },
  // Main app routes - using MainLayout with nested routes
  {
    element: <MainLayout />,
    children: [
      {
        path: '/faq',
        element: <FAQPage />,
      },
      {
        path: '/competition',
        element: <CompetitionPage />,
      },
      {
        path: '/traders',
        element: <AITradersPage />,
      },
      {
        path: '/dashboard',
        element: <TraderDashboard />,
      },
    ],
  },
  {
    path: '*',
    element: <Navigate to="/" replace />,
  },
])
