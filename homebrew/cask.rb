# gitstatus shipped as a cask of prebuilt binaries up to 2.3.0 and is now a
# formula compiled on install. This cask stays in the tap, disabled, so that
# `brew upgrade` tells existing cask installs how to move to the formula. The
# release workflow copies it to the Jleagle/homebrew-gitstatus tap as
# Casks/gitstatus.rb unchanged.
cask "gitstatus" do
  version "2.3.0"

  on_macos do
    on_arm do
      sha256 "8614fbd898791edca336e1ae05000e3f3443729657d88fae6c794bde923c0805"
      url "https://github.com/Jleagle/gitstatus/releases/download/v#{version}/gitstatus_#{version}_darwin_arm64.tar.gz"
    end
    on_intel do
      sha256 "735f8ce755a4bb7d537260c4fcc1b9a67d08b516df77694202ab1e83883758f1"
      url "https://github.com/Jleagle/gitstatus/releases/download/v#{version}/gitstatus_#{version}_darwin_amd64.tar.gz"
    end
  end
  on_linux do
    on_arm do
      sha256 "af7907fdcb42957283d5c16445b71e8efb478c7c790259232cd32aa79c00ffbe"
      url "https://github.com/Jleagle/gitstatus/releases/download/v#{version}/gitstatus_#{version}_linux_arm64.tar.gz"
    end
    on_intel do
      sha256 "774f12df46213ce7449e25bf74aa1b404c28fb0887f98c1068157e9106193f38"
      url "https://github.com/Jleagle/gitstatus/releases/download/v#{version}/gitstatus_#{version}_linux_amd64.tar.gz"
    end
  end

  name "gitstatus"
  desc "Status of every git repo under a directory, with optional pulls"
  homepage "https://github.com/Jleagle/gitstatus"

  disable! date:                "2026-10-07",
           because:             "is now a formula; uninstall this cask, then install the replacement below",
           replacement_formula: "jleagle/gitstatus/gitstatus"

  binary "gitstatus"
end
