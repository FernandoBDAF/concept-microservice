import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "mission-control",
  description: "Cockpit for the microservices lab — visibility, control, experiments",
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
