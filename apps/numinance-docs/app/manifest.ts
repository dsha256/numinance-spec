import type { MetadataRoute } from 'next';

export default function manifest(): MetadataRoute.Manifest {
  return {
    name: 'Numinance',
    short_name: 'Numinance',
    description: 'Give it context. Let it engineer.',
    start_url: '/',
    display: 'standalone',
    background_color: '#0c0c0b',
    theme_color: '#0c0c0b',
    icons: [{ src: '/icons/app-icon-1024-rounded.png', sizes: '1024x1024', type: 'image/png' }],
  };
}
