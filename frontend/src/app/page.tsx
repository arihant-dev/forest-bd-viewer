'use client';

import dynamic from 'next/dynamic';
import { useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { gql } from 'graphql-request';
import { graphqlClient } from '@/lib/graphql';
import { useAppDispatch, useAppSelector } from '@/store';
import { clearUser } from '@/store/authSlice';

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

const LOGOUT_MUTATION = gql`
  mutation Logout {
    logout
  }
`;

export default function Home() {
  const router = useRouter();
  const dispatch = useAppDispatch();
  const { user, loading } = useAppSelector((state) => state.auth);

  useEffect(() => {
    if (!loading && user === null) {
      router.replace('/login');
    }
  }, [user, loading, router]);

  const handleLogout = async () => {
    try {
      await graphqlClient.request(LOGOUT_MUTATION);
    } finally {
      dispatch(clearUser());
      router.replace('/login');
    }
  };

  if (loading || user === null) {
    return (
      <div
        style={{
          width: '100vw',
          height: '100vh',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          background: '#fafafa',
          fontFamily: 'Inter, sans-serif',
        }}
      >
        <p style={{ color: '#6b7280' }}>Loading…</p>
      </div>
    );
  }

  return (
    <div style={{ position: 'relative', width: '100vw', height: '100vh' }}>
      <Map />
      <div
        style={{
          position: 'absolute',
          top: 12,
          right: 12,
          zIndex: 10,
          display: 'flex',
          alignItems: 'center',
          gap: '0.75rem',
          background: 'rgba(255,255,255,0.95)',
          borderRadius: 8,
          padding: '0.5rem 0.75rem',
          boxShadow: '0 2px 8px rgba(0,0,0,0.12)',
          fontFamily: 'Inter, sans-serif',
          fontSize: '0.875rem',
        }}
      >
        <span style={{ color: '#374151' }}>{user.name}</span>
        <button
          onClick={handleLogout}
          style={{
            background: 'none',
            border: '1px solid #d1d5db',
            borderRadius: 6,
            padding: '0.25rem 0.625rem',
            cursor: 'pointer',
            color: '#374151',
            fontSize: '0.8rem',
            fontFamily: 'Inter, sans-serif',
          }}
        >
          Sign out
        </button>
      </div>
    </div>
  );
}
