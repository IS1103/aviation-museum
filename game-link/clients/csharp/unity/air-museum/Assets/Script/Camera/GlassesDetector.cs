// GlassesDetector.cs - 使用 Unity Sentis 執行「是否戴眼鏡」的二元分類
// 支援 iOS（Metal）/ Android（Vulkan）/ Windows / macOS。
// 需 Package：com.unity.sentis。需搭配一個 ONNX 分類器模型（見 README 區塊）。
using System;
using Unity.Sentis;
using UnityEngine;
using UnityEngine.Events;
using UnityEngine.UI;

namespace AirMuseum
{
    public class GlassesDetector : MonoBehaviour
    {
        /// <summary>辨識完成事件：bool = 是否戴眼鏡, float = 該類別機率 (0~1)。</summary>
        [Serializable]
        public class GlassesDetectedEvent : UnityEvent<bool, float> { }

        public enum BackendChoice
        {
            /// <summary>iOS Metal / Windows D3D / Android Vulkan 使用 GPU Compute（推薦）。</summary>
            GPU,
            /// <summary>不支援 Compute 的裝置用 CPU fallback。</summary>
            CPU,
        }

        public enum OutputLayout
        {
            /// <summary>單一輸出（logit），經 sigmoid 後 &gt; Threshold 即判定為「戴眼鏡」。</summary>
            SingleSigmoid,
            /// <summary>兩個輸出 [noGlasses, glasses]，經 softmax 後比較。</summary>
            TwoClassSoftmax,
        }

        [Header("模型")]
        [Tooltip("將 ONNX 模型（例如 glasses_classifier.onnx）拖進來")]
        [SerializeField] private ModelAsset modelAsset;
        [Tooltip("推論後端，iOS 建議 GPU（Metal）")]
        [SerializeField] private BackendChoice backend = BackendChoice.GPU;

        [Header("前處理")]
        [Tooltip("模型的輸入寬度（通常 224）")]
        [SerializeField] private int inputWidth = 224;
        [Tooltip("模型的輸入高度（通常 224）")]
        [SerializeField] private int inputHeight = 224;
        [Tooltip("是否對 RGB 做 ImageNet mean/std 標準化（若 ONNX 已內建標準化請關閉）")]
        [SerializeField] private bool applyImageNetNormalization = true;
        [Tooltip("mean（預設為 ImageNet）")]
        [SerializeField] private Vector3 mean = new Vector3(0.485f, 0.456f, 0.406f);
        [Tooltip("std（預設為 ImageNet）")]
        [SerializeField] private Vector3 std = new Vector3(0.229f, 0.224f, 0.225f);

        [Header("後處理")]
        [SerializeField] private OutputLayout outputLayout = OutputLayout.TwoClassSoftmax;
        [Tooltip("判定為『戴眼鏡』的機率門檻")]
        [Range(0f, 1f)]
        [SerializeField] private float threshold = 0.5f;
        [Tooltip("TwoClassSoftmax 時，『戴眼鏡』類別的 index（一般為 1）")]
        [SerializeField] private int glassesClassIndex = 1;

        [Header("即時推論輸入")]
        [Tooltip("指定 WebCamDisplay，會自動抓當前攝影機畫面做推論")]
        [SerializeField] private WebCamDisplay webCamDisplay;
        [Tooltip("每幾幀推論一次（用於即時模式，iOS 建議 10~30）")]
        [Min(1)]
        [SerializeField] private int inferenceEveryNFrames = 15;
        [Tooltip("是否在 Start 時自動開始即時推論")]
        [SerializeField] private bool autoStartStreaming = false;

        [Header("UI（選填）")]
        [Tooltip("推論結果文字（例：『戴眼鏡 87.3%』）")]
        [SerializeField] private Text resultLabel;

        [Header("事件")]
        [SerializeField] private GlassesDetectedEvent onGlassesDetected;

        private Worker _worker;
        private Model _runtimeModel;
        private RenderTexture _resizedRT;
        private int _frameCounter;
        private bool _streaming;
        private bool _inferring;

        /// <summary>最後一次推論結果：是否戴眼鏡。</summary>
        public bool LastIsWearing { get; private set; }

        /// <summary>最後一次推論結果：戴眼鏡的機率（0~1）。</summary>
        public float LastProbability { get; private set; }

        private void Awake()
        {
            if (modelAsset == null)
            {
                Debug.LogError("[GlassesDetector] 尚未指定 modelAsset（ONNX 模型）。");
                enabled = false;
                return;
            }

            _runtimeModel = ModelLoader.Load(modelAsset);
            BackendType backendType = backend == BackendChoice.GPU
                ? BackendType.GPUCompute
                : BackendType.CPU;
            _worker = new Worker(_runtimeModel, backendType);
        }

        private void Start()
        {
            if (autoStartStreaming) StartStreaming();
        }

        private void OnDestroy()
        {
            _worker?.Dispose();
            _worker = null;

            if (_resizedRT != null)
            {
                _resizedRT.Release();
                _resizedRT = null;
            }
        }

        /// <summary>開始即時串流辨識（每 N 幀從 WebCamDisplay 讀一張推論一次）。</summary>
        public void StartStreaming() => _streaming = true;

        /// <summary>停止即時串流辨識。</summary>
        public void StopStreaming() => _streaming = false;

        private void Update()
        {
            if (!_streaming || _inferring) return;
            if (webCamDisplay == null || !webCamDisplay.IsActive) return;

            _frameCounter++;
            if (_frameCounter < inferenceEveryNFrames) return;
            _frameCounter = 0;

            DetectFrom(webCamDisplay.WebCamTexture);
        }

        /// <summary>
        /// 對一張 Texture（WebCamTexture / Texture2D / RenderTexture）做一次推論。
        /// 可用於「拍照後」的一次性判斷，或即時串流中。
        /// </summary>
        public void DetectFrom(Texture texture)
        {
            if (texture == null || _worker == null) return;
            if (texture.width <= 1 || texture.height <= 1) return;

            _inferring = true;
            try
            {
                EnsureResizeRT();
                // 縮放並轉成固定大小 RGB
                Graphics.Blit(texture, _resizedRT);

                var transform = new TextureTransform()
                    .SetDimensions(inputWidth, inputHeight, 3)
                    .SetTensorLayout(TensorLayout.NCHW);

                using Tensor<float> inputTensor = applyImageNetNormalization
                    ? BuildNormalizedInput(_resizedRT, transform)
                    : TextureConverter.ToTensor(_resizedRT, transform);

                _worker.Schedule(inputTensor);

                using Tensor<float> output = (_worker.PeekOutput() as Tensor<float>)
                    .ReadbackAndClone();

                (bool wearing, float prob) = Postprocess(output);

                LastIsWearing = wearing;
                LastProbability = prob;

                if (resultLabel != null)
                {
                    resultLabel.text = wearing
                        ? $"戴眼鏡 ({prob * 100f:F1}%)"
                        : $"沒戴眼鏡 ({(1f - prob) * 100f:F1}%)";
                }

                onGlassesDetected?.Invoke(wearing, prob);
            }
            finally
            {
                _inferring = false;
            }
        }

        private void EnsureResizeRT()
        {
            if (_resizedRT != null && _resizedRT.width == inputWidth && _resizedRT.height == inputHeight)
                return;

            if (_resizedRT != null) _resizedRT.Release();
            _resizedRT = new RenderTexture(inputWidth, inputHeight, 0, RenderTextureFormat.ARGB32)
            {
                useMipMap = false,
                autoGenerateMips = false,
            };
            _resizedRT.Create();
        }

        /// <summary>
        /// 先把 Texture 轉成 [1,3,H,W] (0~1) Tensor，再用 CPU 套 ImageNet 標準化後上傳新 Tensor。
        /// 對於 224x224 的輸入來說開銷很小（約 150KB 浮點資料），iOS 實測可接受。
        /// </summary>
        private Tensor<float> BuildNormalizedInput(RenderTexture src, TextureTransform transform)
        {
            using Tensor<float> raw = TextureConverter.ToTensor(src, transform);
            using Tensor<float> rawCpu = raw.ReadbackAndClone();

            int c = raw.shape[1];
            int h = raw.shape[2];
            int w = raw.shape[3];
            int hw = h * w;

            float[] data = rawCpu.DownloadToArray();
            float[] m = { mean.x, mean.y, mean.z };
            float[] s = { std.x, std.y, std.z };

            int channels = Mathf.Min(c, 3);
            for (int ci = 0; ci < channels; ci++)
            {
                int offset = ci * hw;
                float mi = m[ci];
                float si = s[ci] == 0f ? 1f : s[ci];
                for (int i = 0; i < hw; i++)
                {
                    data[offset + i] = (data[offset + i] - mi) / si;
                }
            }

            return new Tensor<float>(new TensorShape(1, c, h, w), data);
        }

        private (bool wearing, float prob) Postprocess(Tensor<float> output)
        {
            float[] arr = output.DownloadToArray();

            if (outputLayout == OutputLayout.SingleSigmoid)
            {
                float prob = Sigmoid(arr[0]);
                return (prob >= threshold, prob);
            }

            // TwoClassSoftmax
            if (arr.Length < 2)
            {
                Debug.LogWarning("[GlassesDetector] 模型輸出少於 2，改用 SingleSigmoid 解讀。");
                float prob = Sigmoid(arr[0]);
                return (prob >= threshold, prob);
            }

            float a = arr[0];
            float b = arr[1];
            float maxV = Mathf.Max(a, b);
            float ea = Mathf.Exp(a - maxV);
            float eb = Mathf.Exp(b - maxV);
            float sum = ea + eb;
            float pGlasses = glassesClassIndex == 1 ? (eb / sum) : (ea / sum);
            return (pGlasses >= threshold, pGlasses);
        }

        private static float Sigmoid(float x) => 1f / (1f + Mathf.Exp(-x));
    }
}
