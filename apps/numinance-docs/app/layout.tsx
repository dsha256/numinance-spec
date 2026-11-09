import './globals.css';
import '@fontsource-variable/hanken-grotesk';
import type { Metadata, Viewport } from 'next';
import { GeistMono } from 'geist/font/mono';
import { GeistPixelSquare } from 'geist/font/pixel';
import { RootProvider } from 'fumadocs-ui/provider/next';
import { MotionProvider } from '@/components/motion/motion-provider';
import { siteConfig } from '@/lib/site-config';

export const metadata: Metadata = {
  metadataBase: new URL(siteConfig.url),
  title: { default: siteConfig.title, template: '%s — Numinance' },
  description: siteConfig.description,
  applicationName: 'Numinance',
  alternates: { canonical: '/' },
  icons: {
    icon: [
      { url: '/favicon.ico', sizes: 'any' },
      { url: '/icons/favicon-48.png', type: 'image/png', sizes: '48x48' },
    ],
    apple: '/icons/favicon-180-apple-touch.png',
  },
  openGraph: {
    type: 'website',
    siteName: 'Numinance',
    title: siteConfig.ogTitle,
    description: siteConfig.ogLine,
    images: [{ url: '/og/numinance-og.png', width: 1200, height: 630, alt: 'Numinance — Give it context. Let it engineer.' }],
  },
  twitter: { card: 'summary_large_image', title: siteConfig.ogTitle, description: siteConfig.ogLine, images: ['/og/numinance-og.png'] },
};

export const viewport: Viewport = {
  themeColor: [
    { media: '(prefers-color-scheme: light)', color: '#f2f0ea' },
    { media: '(prefers-color-scheme: dark)', color: '#0c0c0b' },
  ],
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={`${GeistMono.variable} ${GeistPixelSquare.variable}`} suppressHydrationWarning>
      <body className="min-h-screen antialiased">
        <RootProvider theme={{ defaultTheme: 'dark', enableSystem: true }}>
          <MotionProvider>{children}</MotionProvider>
        </RootProvider>
      </body>
    </html>
  );
}
