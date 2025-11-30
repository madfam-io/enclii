/** @type {import('next').NextConfig} */
const nextConfig = {
  output: "standalone",
  env: {
    ENCLII_API_URL: process.env.ENCLII_API_URL || "http://localhost:8001",
  },
};

module.exports = nextConfig;
