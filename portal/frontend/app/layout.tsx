import type { Metadata } from "next";
import type { ReactNode } from "react";
import { Geist, Geist_Mono } from "next/font/google";
import "./globals.css";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: {
    default: "CloudSuite",
    template: "%s · CloudSuite",
  },
  description: "Portal layanan CloudSuite — satu login untuk semua layanan.",
  applicationName: "CloudSuite",
};

/**
 * Set tema SEBELUM paint — mencegah flash.
 * Dark adalah default; hanya berubah ke light jika user simpan "light".
 */
const themeInitScript = `
(function () {
  try {
    var stored = localStorage.getItem("theme");
    var dark = stored !== "light";
    document.documentElement.classList.toggle("dark", dark);
  } catch (e) {
    document.documentElement.classList.add("dark");
  }
})();
`;

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html
      lang="id"
      suppressHydrationWarning
      className={`${geistSans.variable} ${geistMono.variable} h-full antialiased`}
    >
      <head>
        <script dangerouslySetInnerHTML={{ __html: themeInitScript }} />
      </head>
      <body className="min-h-full flex flex-col">{children}</body>
    </html>
  );
}
