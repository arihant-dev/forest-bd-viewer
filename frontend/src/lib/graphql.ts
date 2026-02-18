import { GraphQLClient } from 'graphql-request';

const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

export const graphqlClient = new GraphQLClient(`${API_URL}/graphql`, {
    credentials: 'include', // Send httpOnly cookies
});
