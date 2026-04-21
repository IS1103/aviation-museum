"""Export glasses-detector anyglasses ShuffleNetV2 classifier to ONNX for Unity Sentis.

Upstream: https://github.com/mantasu/glasses-detector (MIT License)

Usage:
    python export_glasses_onnx.py                         # default: medium/anyglasses -> glasses.onnx
    python export_glasses_onnx.py --size small            # tiny classifier (0.12 MB)
    python export_glasses_onnx.py --kind sunglasses       # sunglasses classifier
    python export_glasses_onnx.py --output my_glasses.onnx

Requirements:
    pip install torch torchvision onnx

The exported ONNX has:
    - input:  (1, 3, 256, 256), float32, RGB, values in [0, 255]
      (ImageNet normalization is baked into the graph, so Unity side can feed
       raw [0, 255] RGB pixels directly.)
    - output: (1, 2), float32, softmax probabilities [no_glasses, glasses]
      (matches FaceAnalyzer.RunGlasses() expected format in Unity)
"""

import argparse
import os
import sys
import urllib.request
from pathlib import Path

import torch
import torch.nn as nn
from torchvision.models import shufflenet_v2_x1_0

# ----------------------------------------------------------------------------
# Model info (mirrors glasses_detector.classifier.GlassesClassifier)
# ----------------------------------------------------------------------------
BASE_WEIGHTS_URL = "https://github.com/mantasu/glasses-detector/releases/download"

SIZE_MAP = {
    "small":  {"name": "tinyclsnet_v1",     "version": "v1.0.0"},
    "medium": {"name": "shufflenet_v2_x1_0", "version": "v1.0.0"},
    # "large" uses regnet_x_3_2gf, v1.1.0 - not wired up below for simplicity
}

KINDS = ("anyglasses", "eyeglasses", "sunglasses", "shadows")

# ImageNet normalization (same values used in glasses-detector base_model.predict)
IMAGENET_MEAN = (0.485, 0.456, 0.406)
IMAGENET_STD = (0.229, 0.224, 0.225)


# ----------------------------------------------------------------------------
# Small/tiny architecture (reproduced from glasses_detector.architectures)
# ----------------------------------------------------------------------------
class TinyBinaryClassifier(nn.Module):
    """Reproduction of TinyBinaryClassifier from glasses-detector."""

    def __init__(self):
        super().__init__()
        self.features = nn.Sequential(
            nn.Conv2d(3, 16, 3, 2, 1), nn.ReLU(inplace=True),
            nn.Conv2d(16, 32, 3, 2, 1), nn.ReLU(inplace=True),
            nn.Conv2d(32, 64, 3, 2, 1), nn.ReLU(inplace=True),
            nn.AdaptiveAvgPool2d(1),
        )
        self.classifier = nn.Linear(64, 1)

    def forward(self, x):
        x = self.features(x).flatten(1)
        return self.classifier(x)


def build_base_model(name: str) -> nn.Module:
    if name == "tinyclsnet_v1":
        return TinyBinaryClassifier()
    if name == "shufflenet_v2_x1_0":
        m = shufflenet_v2_x1_0()
        m.fc = nn.Linear(1024, 1)
        return m
    raise ValueError(f"Unsupported architecture: {name}")


# ----------------------------------------------------------------------------
# Export wrapper:
#   - accepts input in [0, 255] RGB (matches Unity FaceAnalyzer.BuildTensor)
#   - runs ImageNet normalization internally
#   - returns (N, 2) softmax probabilities [no_glasses, glasses]
# ----------------------------------------------------------------------------
class ExportWrapper(nn.Module):
    def __init__(self, base: nn.Module):
        super().__init__()
        self.base = base
        mean = torch.tensor(IMAGENET_MEAN, dtype=torch.float32).view(1, 3, 1, 1) * 255.0
        std = torch.tensor(IMAGENET_STD, dtype=torch.float32).view(1, 3, 1, 1) * 255.0
        self.register_buffer("norm_mean", mean)
        self.register_buffer("norm_std", std)

    def forward(self, x: torch.Tensor) -> torch.Tensor:
        x = (x - self.norm_mean) / self.norm_std
        logit = self.base(x)                        # (N, 1)
        p_yes = torch.sigmoid(logit).reshape(-1, 1) # (N, 1)
        p_no = 1.0 - p_yes                          # (N, 1)
        return torch.cat([p_no, p_yes], dim=1)      # (N, 2)


# ----------------------------------------------------------------------------
# Weights download
# ----------------------------------------------------------------------------
def download_weights(kind: str, name: str, version: str, cache_dir: Path) -> Path:
    url = f"{BASE_WEIGHTS_URL}/{version}/classification_{kind}_{name}.pth"
    cache_dir.mkdir(parents=True, exist_ok=True)
    dst = cache_dir / f"classification_{kind}_{name}.pth"
    if dst.exists():
        print(f"[weights] already cached: {dst}")
        return dst

    print(f"[weights] downloading: {url}")
    try:
        urllib.request.urlretrieve(url, dst)
    except Exception as e:
        raise RuntimeError(
            f"Failed to download weights from {url}.\n"
            f"You may download it manually and place it at:\n  {dst}\n"
            f"Original error: {e}"
        )
    print(f"[weights] saved to: {dst}")
    return dst


# ----------------------------------------------------------------------------
# Main
# ----------------------------------------------------------------------------
def main():
    parser = argparse.ArgumentParser(description="Export glasses-detector to ONNX")
    parser.add_argument(
        "--kind", default="anyglasses", choices=KINDS,
        help="which kind of glasses classifier to export (default: anyglasses)"
    )
    parser.add_argument(
        "--size", default="medium", choices=list(SIZE_MAP.keys()),
        help="model size (default: medium → ShuffleNetV2, ~5MB, F1=0.97)"
    )
    parser.add_argument(
        "--output", default="glasses.onnx",
        help="output .onnx file path (default: glasses.onnx)"
    )
    parser.add_argument(
        "--input-size", type=int, default=256,
        help="input spatial size (default: 256, matches training)"
    )
    parser.add_argument(
        "--opset", type=int, default=17,
        help="ONNX opset version (default: 17)"
    )
    parser.add_argument(
        "--cache-dir", default=str(Path.home() / ".cache" / "glasses-detector"),
        help="where to cache downloaded .pth weights"
    )
    args = parser.parse_args()

    info = SIZE_MAP[args.size]
    name = info["name"]
    version = info["version"]

    print(f"[info] kind={args.kind}  size={args.size}  arch={name}  version={version}")

    # 1. Build architecture and load weights
    base = build_base_model(name)
    weights_path = download_weights(args.kind, name, version, Path(args.cache_dir))
    state = torch.load(weights_path, map_location="cpu", weights_only=True)
    base.load_state_dict(state)
    base.eval()

    # 2. Wrap with preprocessing & softmax
    model = ExportWrapper(base).eval()

    # 3. Sanity check forward pass
    dummy = torch.randn(1, 3, args.input_size, args.input_size) * 127.0 + 128.0
    dummy = dummy.clamp(0.0, 255.0)
    with torch.inference_mode():
        out = model(dummy)
    print(f"[sanity] output shape = {tuple(out.shape)}  sample = {out[0].tolist()}")
    assert out.shape == (1, 2), f"expected (1, 2) output, got {tuple(out.shape)}"
    assert abs(out[0].sum().item() - 1.0) < 1e-4, "probabilities should sum to 1"

    # 4. Export to ONNX
    output_path = Path(args.output).resolve()
    output_path.parent.mkdir(parents=True, exist_ok=True)
    print(f"[onnx] exporting to: {output_path}")

    torch.onnx.export(
        model,
        dummy,
        str(output_path),
        input_names=["input"],
        output_names=["probs"],
        # Fixed batch=1 to avoid dynamic shape headaches in Sentis.
        # Height/width are also fixed to input_size for the same reason.
        opset_version=args.opset,
        do_constant_folding=True,
    )

    size_mb = output_path.stat().st_size / (1024 * 1024)
    print(f"[done] wrote {output_path} ({size_mb:.2f} MB)")
    print()
    print("Next steps:")
    print("  1. Move the .onnx file into the Unity project's Assets/Models/")
    print("     (not StreamingAssets - Unity needs to auto-import it as ModelAsset).")
    print("  2. In the Inspector, drag the imported ModelAsset into")
    print("     FaceAnalyzer.glassesModel.")
    print("  3. Make sure FaceAnalyzer.glassesInputSize matches the exported size")
    print(f"     (currently {args.input_size}).")


if __name__ == "__main__":
    try:
        main()
    except Exception as e:
        print(f"[error] {e}", file=sys.stderr)
        sys.exit(1)
