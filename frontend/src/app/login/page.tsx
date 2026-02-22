'use client';

import { useState, FormEvent } from 'react';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { gql } from 'graphql-request';
import { graphqlClient } from '@/lib/graphql';
import { useAppDispatch } from '@/store';
import { setUser, User } from '@/store/authSlice';
import { useI18n } from '@/lib/i18n';
import { useTheme } from '@/lib/theme';

const LOGIN_MUTATION = gql`
  mutation Login($email: String!, $password: String!) {
    login(email: $email, password: $password) {
      user {
        id
        email
        name
      }
    }
  }
`;

export default function LoginPage() {
  const router = useRouter();
  const dispatch = useAppDispatch();
  const { t, toggle: toggleLang } = useI18n();
  const { theme, toggle: toggleTheme } = useTheme();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError('');
    setSubmitting(true);
    try {
      const data = await graphqlClient.request<{ login: { user: User } }>(
        LOGIN_MUTATION,
        { email, password }
      );
      dispatch(setUser(data.login.user));
      router.replace('/');
    } catch {
      setError('Invalid email or password.');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div style={styles.page}>
      {/* Theme + Language toggles */}
      <div style={styles.toggleGroup}>
        <button
          onClick={toggleLang}
          title={t('lang.toggleTitle')}
          style={styles.toggleBtn}
        >
          {t('lang.toggle')}
        </button>
        <button
          onClick={toggleTheme}
          title={theme === 'light' ? t('theme.toggleTitle') : t('theme.toggleDarkTitle')}
          style={styles.toggleBtn}
        >
          {theme === 'light' ? '\u263E' : '\u2600'}
        </button>
      </div>

      <div style={styles.card}>
        <h1 style={styles.title}>{t('auth.appName')}</h1>
        <h2 style={styles.subtitle}>{t('auth.signIn')}</h2>
        <form onSubmit={handleSubmit} style={styles.form}>
          <label style={styles.label}>
            {t('auth.email')}
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              style={styles.input}
              autoComplete="email"
            />
          </label>
          <label style={styles.label}>
            {t('auth.password')}
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              style={styles.input}
              autoComplete="current-password"
            />
          </label>
          {error && <p style={styles.error}>{error}</p>}
          <button type="submit" disabled={submitting} style={styles.button}>
            {submitting ? t('auth.signingIn') : t('auth.signIn')}
          </button>
        </form>
        <p style={styles.footer}>
          {t('auth.noAccount')}{' '}
          <Link href="/register" style={styles.link}>
            {t('auth.register')}
          </Link>
        </p>
      </div>
    </div>
  );
}

const styles: Record<string, React.CSSProperties> = {
  page: {
    minHeight: '100vh',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    background: 'var(--color-bg-auth)',
    fontFamily: 'var(--font-sans)',
  },
  toggleGroup: {
    position: 'fixed',
    top: 16,
    right: 16,
    display: 'flex',
    gap: '0.5rem',
    zIndex: 10,
  },
  toggleBtn: {
    background: 'var(--color-surface)',
    border: '1px solid var(--color-border-medium)',
    borderRadius: '50%',
    width: 36,
    height: 36,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    cursor: 'pointer',
    fontSize: '0.8rem',
    fontWeight: 600,
    color: 'var(--color-text-secondary)',
    boxShadow: 'var(--shadow-sm)',
    transition: 'all 0.2s ease',
  },
  card: {
    background: 'var(--color-surface)',
    borderRadius: 12,
    padding: '2.5rem 2rem',
    boxShadow: 'var(--shadow-card)',
    width: '100%',
    maxWidth: 380,
  },
  title: {
    fontSize: '1.1rem',
    fontWeight: 600,
    color: 'var(--color-accent)',
    margin: '0 0 0.25rem',
    textAlign: 'center' as const,
  },
  subtitle: {
    fontSize: '1.5rem',
    fontWeight: 700,
    color: 'var(--color-text-strong)',
    margin: '0 0 1.5rem',
    textAlign: 'center' as const,
  },
  form: {
    display: 'flex',
    flexDirection: 'column' as const,
    gap: '1rem',
  },
  label: {
    display: 'flex',
    flexDirection: 'column' as const,
    gap: '0.25rem',
    fontSize: '0.875rem',
    fontWeight: 500,
    color: 'var(--color-text-secondary)',
  },
  input: {
    padding: '0.5rem 0.75rem',
    border: '1px solid var(--color-border-medium)',
    borderRadius: 6,
    fontSize: '0.95rem',
    outline: 'none',
    marginTop: '0.25rem',
    background: 'var(--color-surface)',
    color: 'var(--color-text)',
  },
  button: {
    padding: '0.625rem',
    background: 'var(--color-accent)',
    color: '#fff',
    border: 'none',
    borderRadius: 6,
    fontSize: '0.95rem',
    fontWeight: 600,
    cursor: 'pointer',
    marginTop: '0.5rem',
    transition: 'background 0.2s ease, transform 0.15s ease',
  },
  error: {
    color: 'var(--color-error)',
    fontSize: '0.85rem',
    margin: 0,
  },
  footer: {
    textAlign: 'center' as const,
    marginTop: '1.25rem',
    fontSize: '0.875rem',
    color: 'var(--color-text-muted)',
  },
  link: {
    color: 'var(--color-accent)',
    fontWeight: 500,
  },
};
