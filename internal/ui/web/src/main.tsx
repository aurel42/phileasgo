import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { HashRouter, Routes, Route } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import './index.css'
import App from './App.tsx'
import OverlayPage from './OverlayPage.tsx'

const queryClient = new QueryClient()

// Prevent F5/Ctrl+R from reloading the Shell (native wrapper)
document.addEventListener('keydown', function (event) {
  if (event.key === 'F5' || (event.ctrlKey && event.key === 'r')) {
    event.preventDefault();
  }
});

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <HashRouter>
        <Routes>
          <Route path="/" element={<App />} />
          <Route path="/settings" element={<App />} />
          <Route path="/overlay" element={<OverlayPage />} />
        </Routes>
      </HashRouter>
    </QueryClientProvider>
  </StrictMode>,
)
