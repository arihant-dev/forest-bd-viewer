'use client';

import dynamic from 'next/dynamic';
import { useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { gql } from 'graphql-request';
import { graphqlClient } from '@/lib/graphql';
import { useAppDispatch, useAppSelector } from '@/store';
import { clearUser } from '@/store/authSlice';
import { useI18n } from '@/lib/i18n';
import { useTheme } from '@/lib/theme';

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
        background: 'var(--color-bg)',
      }}
    >
      <p style={{ color: 'var(--color-text-muted)', fontFamily: 'var(--font-sans)' }}>
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
  const { t, toggle: toggleLang } = useI18n();
  const { theme, toggle: toggleTheme } = useTheme();

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
          background: 'var(--color-bg)',
          fontFamily: 'var(--font-sans)',
        }}
      >
        <p style={{ color: 'var(--color-text-muted)' }}>Loading…</p>
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
          gap: '0.5rem',
          background: 'var(--color-surface-translucent)',
          backdropFilter: 'blur(8px)',
          WebkitBackdropFilter: 'blur(8px)',
          borderRadius: 'var(--radius)',
          padding: '0.5rem 0.75rem',
          boxShadow: 'var(--shadow-popup)',
          fontFamily: 'var(--font-sans)',
          fontSize: '0.875rem',
        }}
      >
        <span style={{ color: 'var(--color-text-secondary)' }}>{user.name}</span>

        {/* Divider */}
        <span
          style={{
            width: 1,
            height: 18,
            background: 'var(--color-border)',
            flexShrink: 0,
          }}
        />

        {/* Language toggle */}
        <button
          onClick={toggleLang}
          title={t('lang.toggleTitle')}
          style={{
            background: 'none',
            border: '1px solid var(--color-border-medium)',
            borderRadius: 6,
            padding: '0.2rem 0.5rem',
            cursor: 'pointer',
            color: 'var(--color-primary)',
            fontSize: '0.75rem',
            fontWeight: 600,
            fontFamily: 'var(--font-sans)',
            letterSpacing: '0.03em',
            transition: 'all 0.2s ease',
          }}
        >
          {t('lang.toggle')}
        </button>

        {/* Theme toggle */}
        <button
          onClick={toggleTheme}
          title={theme === 'light' ? t('theme.toggleTitle') : t('theme.toggleDarkTitle')}
          style={{
            background: 'none',
            border: '1px solid var(--color-border-medium)',
            borderRadius: 6,
            padding: '0.2rem 0.5rem',
            cursor: 'pointer',
            color: 'var(--color-text-secondary)',
            fontSize: '0.8rem',
            fontFamily: 'var(--font-sans)',
            lineHeight: 1,
            transition: 'all 0.2s ease',
          }}
        >
          {theme === 'light' ? '\u263E' : '\u2600'}
        </button>

        {/* Sign out */}
        <button
          onClick={handleLogout}
          style={{
            background: 'none',
            border: '1px solid var(--color-border-medium)',
            borderRadius: 6,
            padding: '0.25rem 0.625rem',
            cursor: 'pointer',
            color: 'var(--color-text-secondary)',
            fontSize: '0.8rem',
            fontFamily: 'var(--font-sans)',
            transition: 'all 0.2s ease',
          }}
        >
          {t('auth.signOut')}
        </button>
      </div>
    </div>
  );
}
