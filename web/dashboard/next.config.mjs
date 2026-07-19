/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  // The control-plane API base (Part 05). Server-only; the browser never calls
  // it directly — reads go through Server Components and writes through Server
  // Actions, so the JWT stays server-side.
  env: {
    RIFT_API_URL: process.env.RIFT_API_URL || "http://localhost:8080",
  },
};

export default nextConfig;
