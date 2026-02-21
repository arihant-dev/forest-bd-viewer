'use client';

import { useAppDispatch, useAppSelector } from '@/store';
import { clearAnalysis } from '@/store/analysisSlice';
import { useI18n, type DictKey } from '@/lib/i18n';
import styles from './AnalysisPanel.module.css';

const TFV_COLORS: Record<string, string> = {
    FF1: '#1a7a3c',
    FF2: '#2d9e55',
    FF3: '#52b788',
    FF4: '#40916c',
    FO1: '#74c69d',
    FO2: '#95d5b2',
    FO3: '#b7e4c7',
    LA: '#d4e157',
    FP: '#aed9c9',
};
const DEFAULT_COLOR = '#a8d5a2';

function fmt(n: number, decimals = 1): string {
    return n.toLocaleString('en-US', {
        minimumFractionDigits: decimals,
        maximumFractionDigits: decimals,
    });
}

interface Props {
    onClear: () => void;
}

export default function AnalysisPanel({ onClear }: Props) {
    const dispatch = useAppDispatch();
    const { status, result, error } = useAppSelector((s) => s.analysis);
    const { t } = useI18n();

    if (status === 'idle') return null;

    const handleClose = () => {
        dispatch(clearAnalysis());
        onClear();
    };

    return (
        <div className={styles.panel}>
            <div className={styles.header}>
                <span className={styles.title}>{t('analysis.title')}</span>
                <button className={styles.closeBtn} onClick={handleClose} aria-label={t('analysis.close')}>
                    ✕
                </button>
            </div>

            {status === 'drawing' && (
                <p className={styles.hint}>{t('analysis.hint')}</p>
            )}

            {status === 'loading' && (
                <div className={styles.loadingRow}>
                    <span className={styles.spinnerSmall} />
                    <span>{t('analysis.loading')}</span>
                </div>
            )}

            {status === 'error' && (
                <p className={styles.error}>{error ?? t('analysis.error')}</p>
            )}

            {status === 'done' && result && (
                <>
                    <div className={styles.metrics}>
                        <div className={styles.metric}>
                            <span className={styles.metricValue}>{fmt(result.areaHa)} ha</span>
                            <span className={styles.metricLabel}>{t('analysis.totalArea')}</span>
                        </div>
                        <div className={styles.metric}>
                            <span className={styles.metricValue}>{fmt(result.forestCoverHa)} ha</span>
                            <span className={styles.metricLabel}>{t('analysis.forestCover')}</span>
                        </div>
                        <div className={styles.metric}>
                            <span className={styles.metricValue}>{fmt(result.forestCoverPct, 0)} %</span>
                            <span className={styles.metricLabel}>{t('analysis.forestShare')}</span>
                        </div>
                        <div className={styles.metric}>
                            <span className={styles.metricValue}>{result.parcelCount}</span>
                            <span className={styles.metricLabel}>{t('analysis.parcels')}</span>
                        </div>
                    </div>

                    <div className={styles.coverBar}>
                        <div
                            className={styles.coverBarFill}
                            style={{ width: `${Math.min(result.forestCoverPct, 100)}%` }}
                        />
                    </div>

                    {result.tfvBreakdown.length > 0 && (
                        <section className={styles.section}>
                            <h4 className={styles.sectionTitle}>{t('analysis.tfvSection')}</h4>
                            {result.tfvBreakdown.map((row) => {
                                const tfvKey = `tfv.${row.codeTfv}` as DictKey;
                                const translated = t(tfvKey);
                                const label = translated.startsWith('tfv.') ? row.libTfv : translated;
                                return (
                                <div key={row.codeTfv} className={styles.breakdownRow}>
                                    <span
                                        className={styles.swatch}
                                        style={{ background: TFV_COLORS[row.codeTfv] ?? DEFAULT_COLOR }}
                                    />
                                    <span className={styles.breakdownLabel} title={label}>
                                        {label}
                                    </span>
                                    <span className={styles.breakdownBar}>
                                        <span
                                            className={styles.breakdownBarFill}
                                            style={{
                                                width: `${row.pct}%`,
                                                background: TFV_COLORS[row.codeTfv] ?? DEFAULT_COLOR,
                                            }}
                                        />
                                    </span>
                                    <span className={styles.breakdownPct}>{fmt(row.pct, 0)} %</span>
                                </div>
                                );
                            })}
                        </section>
                    )}

                    {result.speciesBreakdown.length > 0 && (
                        <section className={styles.section}>
                            <h4 className={styles.sectionTitle}>{t('analysis.speciesSection')}</h4>
                            {result.speciesBreakdown.slice(0, 8).map((row) => (
                                <div key={row.essence} className={styles.breakdownRow}>
                                    <span className={styles.swatchNeutral} />
                                    <span className={styles.breakdownLabel}>{row.essence}</span>
                                    <span className={styles.breakdownBar}>
                                        <span
                                            className={styles.breakdownBarFill}
                                            style={{ width: `${row.pct}%`, background: '#52b788' }}
                                        />
                                    </span>
                                    <span className={styles.breakdownPct}>{fmt(row.pct, 0)} %</span>
                                </div>
                            ))}
                        </section>
                    )}

                    {result.parcelCount === 0 && (
                        <p className={styles.hint}>{t('analysis.noParcels')}</p>
                    )}
                </>
            )}
        </div>
    );
}
