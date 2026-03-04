import type { Metadata } from "next";
import "./globals.css";
import "./theme-dark.css";
import "./theme-light.css";
import "./components.css";
import Sidebar from "@/components/layout/Sidebar";
import ChatPanel from "@/components/chat/ChatPanel";

export const metadata: Metadata = {
  title: "AI-CM | Category Manager Copilot",
  description: "AI-powered decision intelligence for Category Managers",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className="dark">
      <head>
        <link
          href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&display=swap"
          rel="stylesheet"
        />
      </head>
      <body>
        <div className="app-shell">
          <Sidebar />
          <main className="main-content">
            {children}
          </main>
          <ChatPanel />
        </div>
      </body>
    </html>
  );
}
