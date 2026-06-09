# NOTE: This formula is informational only.
# GoReleaser auto-generates the Homebrew Cask during release
# using the config in .goreleaser/cli.yaml.
# See https://github.com/ammiranda/homebrew-tap for the actual tap.

class OtfCli < Formula
  desc "CLI for the OrangeTheory Fitness API — browse, book, and manage OTF classes"
  homepage "https://github.com/ammiranda/otf_api"
  license "MIT"

  stable do
    version "0.1.0"

    on_macos do
      on_arm do
        url "https://github.com/ammiranda/otf_api/releases/download/otf-cli/v#{version}/otf-cli-darwin-arm64"
        sha256 "PLACEHOLDER_DARWIN_ARM64"
      end
      on_intel do
        url "https://github.com/ammiranda/otf_api/releases/download/otf-cli/v#{version}/otf-cli-darwin-amd64"
        sha256 "PLACEHOLDER_DARWIN_AMD64"
      end
    end

    on_linux do
      on_arm do
        url "https://github.com/ammiranda/otf_api/releases/download/otf-cli/v#{version}/otf-cli-linux-arm64"
        sha256 "PLACEHOLDER_LINUX_ARM64"
      end
      on_intel do
        url "https://github.com/ammiranda/otf_api/releases/download/otf-cli/v#{version}/otf-cli-linux-amd64"
        sha256 "PLACEHOLDER_LINUX_AMD64"
      end
    end
  end

  def install
    downloaded = Dir["otf-cli-*"].first
    bin.install downloaded => "otf-cli"
  end

  test do
    assert_match "otf-cli", shell_output("#{bin}/otf-cli --help")
  end
end
