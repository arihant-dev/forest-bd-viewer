'use client';

import dynamic from 'next/dynamic';

const Map = dynamic(() => import('@/components/Map'), {
  ssr: false,
  loading: () => (
    <div
      style={{
        width: '100vw',
        height: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: '#fafafa',
      }}
    >
      <p style={{ color: '#6b7280', fontFamily: 'Inter, sans-serif' }}>
        Loading map…
      </p>
    </div>
  ),
});

export default function Home() {
  return <Map />;
}
