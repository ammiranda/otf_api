# NOTE: This formula is informational only.
# GoReleaser auto-generates the Homebrew Cask during release
# using the config in .goreleaser/mcp.yaml.
# See https://github.com/ammiranda/homebrew-tap for the actual tap.

class OtfMcp < Formula
  desc "MCP server for the OrangeTheory Fitness API — AI-ready class booking"
  homepage "https://github.com/ammiranda/otf_api"
  license "MIT"

  stable do
    version "0.1.0"

    on_macos do
      on_arm do
        url "https://github.com/ammiranda/otf_api/releases/download/otf-mcp/v#{version}/otf-mcp-darwin-arm64"
        sha256 "PLACEHOLDER_DARWIN_ARM64"
      end
      on_intel do
        url "https://github.com/ammiranda/otf_api/releases/download/otf-mcp/v#{version}/otf-mcp-darwin-amd64"
        sha256 "PLACEHOLDER_DARWIN_AMD64"
      end
    end

    on_linux do
      on_arm do
        url "https://github.com/ammiranda/otf_api/releases/download/otf-mcp/v#{version}/otf-mcp-linux-arm64"
        sha256 "PLACEHOLDER_LINUX_ARM64"
      end
      on_intel do
        url "https://github.com/ammiranda/otf_api/releases/download/otf-mcp/v#{version}/otf-mcp-linux-amd64"
        sha256 "PLACEHOLDER_LINUX_AMD64"
      end
    end
  end

  def install
    downloaded = Dir["otf-mcp-*"].first
    bin.install downloaded => "otf-mcp"
  end

  test do
    assert_match "otf-mcp", shell_output("#{bin}/otf-mcp --version")
  end
end
