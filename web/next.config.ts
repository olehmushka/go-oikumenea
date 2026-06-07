import type { NextConfig } from "next";

// Standalone output keeps the production image lean (D-WebUI packaging).
const nextConfig: NextConfig = {
  output: "standalone",
  // No ESLint config is shipped; don't gate the production build on it (types are still checked).
  eslint: { ignoreDuringBuilds: true },
};

export default nextConfig;
