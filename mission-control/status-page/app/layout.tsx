import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "lab-status",
  description: "Read-only status console for the microservices lab",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
