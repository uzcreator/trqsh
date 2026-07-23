/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  // The control-plane API base (Part 05). Server-only; the browser never calls
  // it directly — reads go through Server Components and writes through Server
  // Actions, so the JWT stays server-side.
  env: {
    TRQSH_API_URL: process.env.TRQSH_API_URL || "http://localhost:8080",
  },
};

export default nextConfig;
