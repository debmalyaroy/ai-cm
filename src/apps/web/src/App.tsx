import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom';
import Sidebar from './components/layout/Sidebar';
import ChatPanel from './components/chat/ChatPanel';

import Dashboard from './pages/Dashboard';
import Alerts from './pages/Alerts';
import Actions from './pages/Actions';
import News from './pages/News';
import Reports from './pages/Reports';
import Config from './pages/Config';
import ChatPage from './pages/Chat';

import './globals.css';
import './theme-dark.css';
import './theme-light.css';
import './components.css';

function App() {
  return (
    <Router>
      <div className="dark app-shell font-sans min-h-screen bg-app-bg text-gray-100 flex overflow-hidden">
        <Sidebar />

        <main className="main-content flex-grow flex flex-col min-w-0 bg-app-bg overflow-hidden relative border-x border-[#2A2A2A]">
          <Routes>
            {/* Redirect root to dashboard */}
            <Route path="/" element={<Navigate to="/dashboard" replace />} />

            <Route path="/dashboard" element={<Dashboard />} />
            <Route path="/alerts" element={<Alerts />} />
            <Route path="/actions" element={<Actions />} />
            <Route path="/news" element={<News />} />
            <Route path="/reports" element={<Reports />} />
            <Route path="/config" element={<Config />} />
            <Route path="/chat" element={<ChatPage />} />

            <Route path="*" element={<div className="p-8"><h1>404 Not Found</h1></div>} />
          </Routes>
        </main>

        <ChatPanel />
      </div>
    </Router>
  );
}

export default App;
