import { createSlice, createAsyncThunk } from '@reduxjs/toolkit';
import { gql } from 'graphql-request';
import { graphqlClient } from '@/lib/graphql';

// ── Types ────────────────────────────────────────────────────────────────────

export interface TfvBreakdown {
    codeTfv: string;
    libTfv: string;
    areaHa: number;
    pct: number;
}

export interface SpeciesBreakdown {
    essence: string;
    areaHa: number;
    pct: number;
}

export interface PolygonAnalysis {
    areaHa: number;
    forestCoverHa: number;
    forestCoverPct: number;
    parcelCount: number;
    tfvBreakdown: TfvBreakdown[];
    speciesBreakdown: SpeciesBreakdown[];
}

export interface LidarAnalysis {
    hasCoverage: boolean;
    message?: string;
    minHeight?: number;
    maxHeight?: number;
    meanHeight?: number;
    medianHeight?: number;
    chmImageUrl?: string;
    bounds?: number[];
}

export type AnalysisStatus = 'idle' | 'drawing' | 'loading' | 'done' | 'error';
export type LidarStatus = 'idle' | 'loading' | 'done' | 'error';

interface AnalysisState {
    status: AnalysisStatus;
    result: PolygonAnalysis | null;
    error: string | null;
    lidarStatus: LidarStatus;
    lidarResult: LidarAnalysis | null;
    lidarError: string | null;
}

// ── GraphQL ──────────────────────────────────────────────────────────────────

const ANALYZE_POLYGON_MUTATION = gql`
    mutation AnalyzePolygon($geojson: String!) {
        analyzePolygon(geojson: $geojson) {
            areaHa
            forestCoverHa
            forestCoverPct
            parcelCount
            tfvBreakdown {
                codeTfv
                libTfv
                areaHa
                pct
            }
            speciesBreakdown {
                essence
                areaHa
                pct
            }
        }
    }
`;

const ANALYZE_LIDAR_MUTATION = gql`
    mutation AnalyzeLidar($geojson: String!) {
        analyzeLidar(geojson: $geojson) {
            hasCoverage
            message
            minHeight
            maxHeight
            meanHeight
            medianHeight
            chmImageUrl
            bounds
        }
    }
`;

// ── Thunks ───────────────────────────────────────────────────────────────────

export const analyzePolygonThunk = createAsyncThunk(
    'analysis/analyze',
    async (geojson: string, { rejectWithValue }) => {
        try {
            const data = await graphqlClient.request<{ analyzePolygon: PolygonAnalysis }>(
                ANALYZE_POLYGON_MUTATION,
                { geojson }
            );
            return data.analyzePolygon;
        } catch (err: unknown) {
            const msg =
                err instanceof Error
                    ? err.message
                    : 'Analysis failed';
            return rejectWithValue(msg);
        }
    }
);

export const analyzeLidarThunk = createAsyncThunk(
    'analysis/lidar',
    async (geojson: string, { rejectWithValue }) => {
        try {
            const data = await graphqlClient.request<{ analyzeLidar: LidarAnalysis }>(
                ANALYZE_LIDAR_MUTATION,
                { geojson }
            );
            return data.analyzeLidar;
        } catch (err: unknown) {
            const msg =
                err instanceof Error
                    ? err.message
                    : 'LiDAR analysis failed';
            return rejectWithValue(msg);
        }
    }
);

// ── Slice ────────────────────────────────────────────────────────────────────

const analysisSlice = createSlice({
    name: 'analysis',
    initialState: {
        status: 'idle',
        result: null,
        error: null,
        lidarStatus: 'idle',
        lidarResult: null,
        lidarError: null,
    } as AnalysisState,
    reducers: {
        startDrawing(state) {
            state.status = 'drawing';
            state.result = null;
            state.error = null;
            state.lidarStatus = 'idle';
            state.lidarResult = null;
            state.lidarError = null;
        },
        clearAnalysis(state) {
            state.status = 'idle';
            state.result = null;
            state.error = null;
            state.lidarStatus = 'idle';
            state.lidarResult = null;
            state.lidarError = null;
        },
    },
    extraReducers: (builder) => {
        builder
            .addCase(analyzePolygonThunk.pending, (state) => {
                state.status = 'loading';
                state.result = null;
                state.error = null;
            })
            .addCase(analyzePolygonThunk.fulfilled, (state, action) => {
                state.status = 'done';
                state.result = action.payload;
            })
            .addCase(analyzePolygonThunk.rejected, (state, action) => {
                state.status = 'error';
                state.error = (action.payload as string) ?? 'Unknown error';
            })
            // LiDAR thunk
            .addCase(analyzeLidarThunk.pending, (state) => {
                state.lidarStatus = 'loading';
                state.lidarResult = null;
                state.lidarError = null;
            })
            .addCase(analyzeLidarThunk.fulfilled, (state, action) => {
                state.lidarStatus = 'done';
                state.lidarResult = action.payload;
            })
            .addCase(analyzeLidarThunk.rejected, (state, action) => {
                state.lidarStatus = 'error';
                state.lidarError = (action.payload as string) ?? 'Unknown error';
            });
    },
});

export const { startDrawing, clearAnalysis } = analysisSlice.actions;
export default analysisSlice.reducer;
