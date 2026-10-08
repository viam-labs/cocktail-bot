import "./globals.css";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Cocktail bot",
  description: "Order cocktails from a Viam-powered bar",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body className="m-0 bg-white">{children}</body>
    </html>
  );
}
