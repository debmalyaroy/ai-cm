import type { Metadata } from "next";

export const metadata: Metadata = {
    title: "Sign In | AI-CM Category Manager Copilot",
    description: "Sign in to AI-CM Category Manager Copilot",
};

export default function LoginLayout({
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
            <body style={{ margin: 0, padding: 0 }}>{children}</body>
        </html>
    );
}
