require "language/go"

class Gitstatus < Formula
  desc "Summarises your repos and help to update them"
  homepage "https://github.com/Jleagle/gitstatus"
  url "https://github.com/Jleagle/gitstatus/archive/refs/tags/v1.0.0.tar.gz"
  license "MIT"
  head "https://github.com/Jleagle/gitstatus.git"
  branch: "main"
  depends_on "go" => :build

  def install
    system "go", "install", "github.com/Jleagle/gitstatus@latest"
  end

  test do
    system "true"
  end
end
