import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter, Routes, Route } from 'react-router-dom'
import Week from './Week'
import Crew from './Crew'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Week />} />
        <Route path="/crew/:name" element={<Crew />} />
        <Route path="*" element={<Week />} />
      </Routes>
    </BrowserRouter>
  </StrictMode>,
)
