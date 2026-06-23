import type { NextConfig } from "next";
import path from "node:path";

// Standalone output keeps the production image lean (D-WebUI packaging). The typed SDK
// (oikumenea-client) is a local file: dependency living OUTSIDE web/ (../clients/typescript), so the
// output-file tracer must root at the repo so it follows the symlink into the standalone bundle.
const nextConfig: NextConfig = {
  output: "standalone",
  outputFileTracingRoot: path.join(import.meta.dirname, ".."),
  // No ESLint config is shipped; don't gate the production build on it (types are still checked).
  eslint: { ignoreDuringBuilds: true },
};

export default nextConfig;
