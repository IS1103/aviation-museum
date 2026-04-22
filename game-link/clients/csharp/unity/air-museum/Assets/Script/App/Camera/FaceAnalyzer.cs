// FaceAnalyzer.cs - 使用 Sentis / Unity Inference Engine 2.x 進行臉部屬性分析
// 目前支援：性別、年齡（InsightFace genderage.onnx）
// 眼鏡偵測：需要額外的分類模型（InsightFace 這批沒有），目前留 TODO
//
// 模型匯入說明：
//   .onnx 檔必須放在「Assets/」底下（不能在 StreamingAssets），Unity 才會自動
//   匯入成 ModelAsset。建議建立 Assets/Models/ 資料夾，把 .onnx 丟進去後，
//   在 Inspector 中把匯入好的 ModelAsset 拖入此元件。
using System;
using Unity.InferenceEngine;
using UnityEngine;

namespace AirMuseum
{
    public class FaceAnalyzer : MonoBehaviour
    {
        public enum Gender
        {
            Female = 0,
            Male = 1,
        }

        public struct AnalysisResult
        {
            /// <summary>是否成功取得推論結果。</summary>
            public bool success;
            public Gender gender;
            /// <summary>年齡（0~120，估計值）。</summary>
            public int age;
            /// <summary>是否有戴眼鏡。目前沒有對應模型，永遠為 false。</summary>
            public bool wearsGlasses;
            /// <summary>眼鏡判斷是否可用（有載入眼鏡模型時才為 true）。</summary>
            public bool glassesAvailable;
        }

        [Header("Sentis 模型")]
        [Tooltip("InsightFace genderage.onnx（匯入後的 ModelAsset），輸出性別+年齡")]
        [SerializeField] private ModelAsset genderAgeModel;

        [Tooltip("（選填）眼鏡分類模型。推薦使用 Tools/export_glasses_onnx.py 匯出的 glasses.onnx，\n" +
                 "輸入 (1,3,256,256) RGB [0,255]、輸出 (1,2) softmax [no, yes]。")]
        [SerializeField] private ModelAsset glassesModel;

        [Header("推論設定")]
        [Tooltip("推論使用的後端。預設 CPU（Burst）：模型很小、CPU 速度足夠，且可避開部分 DX11 compute shader 的已知 bug。" +
                 "若要改 GPU 推論，可改成 GPUCompute 或 GPUPixel；若 GPUCompute 初始化失敗會自動退回 CPU。")]
        [SerializeField] private BackendType backend = BackendType.CPU;

        [Tooltip("genderage 模型的輸入尺寸（InsightFace 預設 96）")]
        [SerializeField] private int genderAgeInputSize = 96;

        [Tooltip("眼鏡分類模型的輸入尺寸（glasses-detector 預設 256）")]
        [SerializeField] private int glassesInputSize = 256;

        [Tooltip("除了 gender/age 之外，列印原始輸出張量值以便除錯")]
        [SerializeField] private bool verboseLog = false;

        private Worker _genderAgeWorker;
        private Worker _glassesWorker;
        private Model _genderAgeModel;
        private Model _glassesModel;
        private BackendType _activeBackend;

        public bool IsReady => _genderAgeWorker != null;

        private void Awake()
        {
            _activeBackend = backend;
            _genderAgeModel = TryLoadModel(genderAgeModel, "genderage");
            _glassesModel = TryLoadModel(glassesModel, "glasses");
            RebuildWorkers(_activeBackend);
        }

        private void OnDestroy()
        {
            _genderAgeWorker?.Dispose();
            _genderAgeWorker = null;
            _glassesWorker?.Dispose();
            _glassesWorker = null;
        }

        private static Model TryLoadModel(ModelAsset asset, string label)
        {
            if (asset == null) return null;
            try
            {
                return ModelLoader.Load(asset);
            }
            catch (Exception e)
            {
                Debug.LogError($"[FaceAnalyzer] {label} 模型載入失敗：{e.Message}");
                return null;
            }
        }

        private void RebuildWorkers(BackendType backendType)
        {
            _genderAgeWorker?.Dispose();
            _genderAgeWorker = null;
            _glassesWorker?.Dispose();
            _glassesWorker = null;

            if (_genderAgeModel != null && TryNewWorker(_genderAgeModel, backendType, out var ga))
            {
                _genderAgeWorker = ga;
                Debug.Log($"[FaceAnalyzer] genderage Worker 建立成功（backend: {backendType}）");
            }

            if (_glassesModel != null && TryNewWorker(_glassesModel, backendType, out var gl))
            {
                _glassesWorker = gl;
                Debug.Log($"[FaceAnalyzer] glasses Worker 建立成功（backend: {backendType}）");
            }

            _activeBackend = backendType;
        }

        /// <summary>
        /// 遇到 GPU compute shader 相容性問題（例如 Unity.InferenceEngine.ComputeFunctions 的
        /// TypeInitializationException / IndexOutOfRangeException）時呼叫此方法改用 CPU 重建 Worker。
        /// 回傳是否成功 fallback。
        /// </summary>
        private bool FallbackToCPUIfNeeded(Exception e)
        {
            if (_activeBackend == BackendType.CPU) return false;
            if (!IsComputeShaderInitException(e)) return false;

            Debug.LogWarning($"[FaceAnalyzer] {_activeBackend} 推論時 compute shader 初始化失敗，改用 CPU 重試：{e.Message}");
            RebuildWorkers(BackendType.CPU);
            return _genderAgeWorker != null;
        }

        private static bool IsComputeShaderInitException(Exception e)
        {
            for (var cur = e; cur != null; cur = cur.InnerException)
            {
                if (cur is TypeInitializationException) return true;
                if (cur is IndexOutOfRangeException &&
                    cur.StackTrace != null &&
                    cur.StackTrace.Contains("ComputeFunction")) return true;
            }
            return false;
        }

        private static bool TryNewWorker(Model model, BackendType backendType, out Worker worker)
        {
            try
            {
                worker = new Worker(model, backendType);
                return true;
            }
            catch (Exception e)
            {
                Debug.LogWarning($"[FaceAnalyzer] 建立 Worker 失敗（backend={backendType}）：{e.Message}");
                worker = null;
                return false;
            }
        }

        /// <summary>
        /// 分析照片中的人臉屬性（性別、年齡、戴眼鏡）。
        /// 為了簡化目前沒有做人臉偵測，會直接中央裁切正方形後送入模型；
        /// 自拍類的應用通常人臉位於中央，這在多數情況下可用。
        /// </summary>
        public AnalysisResult Analyze(Texture2D photo)
        {
            var result = new AnalysisResult();

            if (photo == null)
            {
                Debug.LogWarning("[FaceAnalyzer] 輸入照片為 null。");
                return result;
            }

            if (_genderAgeWorker == null)
            {
                Debug.LogWarning("[FaceAnalyzer] genderage 模型尚未載入，請在 Inspector 指派 Model Asset。");
                return result;
            }

            // 各模型輸入尺寸不同，分別裁切
            Texture2D genderAgeCrop = null;
            Texture2D glassesCrop = null;

            try
            {
                genderAgeCrop = CenterSquareCropAndResize(photo, genderAgeInputSize);
                if (!RunWithFallback(RunGenderAgeStep, genderAgeCrop, ref result, "genderage"))
                    return result;

                if (_glassesWorker != null)
                {
                    glassesCrop = CenterSquareCropAndResize(photo, glassesInputSize);
                    RunWithFallback(RunGlassesStep, glassesCrop, ref result, "glasses");
                }

                result.success = true;
            }
            finally
            {
                if (genderAgeCrop != null) Destroy(genderAgeCrop);
                if (glassesCrop != null) Destroy(glassesCrop);
            }

            return result;
        }

        private delegate void InferStep(Texture2D faceCrop, ref AnalysisResult result);

        private void RunGenderAgeStep(Texture2D faceCrop, ref AnalysisResult result) => RunGenderAge(faceCrop, ref result);
        private void RunGlassesStep(Texture2D faceCrop, ref AnalysisResult result) => RunGlasses(faceCrop, ref result);

        /// <summary>
        /// 執行一段推論；若遇到 GPU compute shader 初始化例外，會自動改用 CPU 後端重建 Worker 再重試一次。
        /// </summary>
        private bool RunWithFallback(InferStep step, Texture2D faceCrop, ref AnalysisResult result, string label)
        {
            try
            {
                step(faceCrop, ref result);
                return true;
            }
            catch (Exception e)
            {
                if (!FallbackToCPUIfNeeded(e))
                {
                    Debug.LogError($"[FaceAnalyzer] {label} 推論失敗：{e}");
                    return false;
                }
            }

            if (_genderAgeWorker == null) return false;

            try
            {
                step(faceCrop, ref result);
                return true;
            }
            catch (Exception e2)
            {
                Debug.LogError($"[FaceAnalyzer] {label} CPU fallback 後仍推論失敗：{e2}");
                return false;
            }
        }

        private void RunGenderAge(Texture2D faceCrop, ref AnalysisResult result)
        {
            int size = genderAgeInputSize;

            // InsightFace genderage 模型：
            //   input shape : (1, 3, size, size)
            //   layout      : NCHW
            //   channel     : RGB
            //   value range : [0, 255]（未做 mean/std 正規化）
            //   output shape: (1, 3) → [female_score, male_score, age/100]
            using var input = BuildTensor(faceCrop, size, size);

            _genderAgeWorker.Schedule(input);

            var output = _genderAgeWorker.PeekOutput() as Tensor<float>;
            if (output == null)
            {
                Debug.LogWarning("[FaceAnalyzer] genderage 輸出型別不符。");
                return;
            }

            float[] data = output.DownloadToArray();
            if (data.Length < 3)
            {
                Debug.LogWarning($"[FaceAnalyzer] genderage 輸出長度不足：{data.Length}");
                return;
            }

            if (verboseLog)
            {
                Debug.Log($"[FaceAnalyzer] genderage raw = [{data[0]:F3}, {data[1]:F3}, {data[2]:F3}]");
            }

            result.gender = data[1] > data[0] ? Gender.Male : Gender.Female;
            result.age = Mathf.Clamp(Mathf.RoundToInt(data[2] * 100f), 0, 120);
        }

        private void RunGlasses(Texture2D faceCrop, ref AnalysisResult result)
        {
            if (_glassesWorker == null)
            {
                result.glassesAvailable = false;
                result.wearsGlasses = false;
                return;
            }

            // 預期的模型規格（與 Tools/export_glasses_onnx.py 匯出的 ONNX 對齊）：
            //   input  : (1, 3, 256, 256) RGB，值 [0, 255]
            //            ImageNet 正規化 (x/255 - mean) / std 已寫進 ONNX graph 裡
            //   output : (1, 2) softmax [p_no_glasses, p_yes_glasses]
            using var input = BuildTensor(faceCrop, faceCrop.width, faceCrop.height);
            _glassesWorker.Schedule(input);

            var output = _glassesWorker.PeekOutput() as Tensor<float>;
            if (output == null)
            {
                Debug.LogWarning("[FaceAnalyzer] glasses 輸出型別不符。");
                return;
            }

            float[] data = output.DownloadToArray();
            if (data.Length < 2)
            {
                Debug.LogWarning($"[FaceAnalyzer] glasses 輸出長度不足：{data.Length}");
                return;
            }

            result.glassesAvailable = true;
            result.wearsGlasses = data[1] > data[0];
            if (verboseLog)
            {
                Debug.Log($"[FaceAnalyzer] glasses raw = [no={data[0]:F3}, yes={data[1]:F3}]");
            }
        }

        // =====================================================================
        // 影像前處理
        // =====================================================================

        /// <summary>
        /// 從原圖中央裁成正方形，縮放到指定邊長。回傳一張新的可讀 Texture2D（呼叫端負責 Destroy）。
        /// </summary>
        private static Texture2D CenterSquareCropAndResize(Texture2D src, int targetSize)
        {
            int minSide = Mathf.Min(src.width, src.height);
            int cropX = (src.width - minSide) / 2;
            int cropY = (src.height - minSide) / 2;

            // 用 RenderTexture 做 crop + resize
            var rt = RenderTexture.GetTemporary(targetSize, targetSize, 0, RenderTextureFormat.ARGB32);
            var prevActive = RenderTexture.active;

            Vector2 scale = new Vector2((float)minSide / src.width, (float)minSide / src.height);
            Vector2 offset = new Vector2((float)cropX / src.width, (float)cropY / src.height);
            Graphics.Blit(src, rt, scale, offset);

            RenderTexture.active = rt;
            var dst = new Texture2D(targetSize, targetSize, TextureFormat.RGB24, false);
            dst.ReadPixels(new Rect(0, 0, targetSize, targetSize), 0, 0);
            dst.Apply(false);
            RenderTexture.active = prevActive;
            RenderTexture.ReleaseTemporary(rt);

            return dst;
        }

        /// <summary>
        /// 把一張 Texture2D 轉成 NCHW、RGB、值域 [0, 255] 的 Tensor&lt;float&gt;。
        /// </summary>
        private static Tensor<float> BuildTensor(Texture2D tex, int targetW, int targetH)
        {
            Texture2D source = tex;
            bool ownSource = false;
            if (tex.width != targetW || tex.height != targetH)
            {
                source = CenterSquareCropAndResize(tex, Mathf.Max(targetW, targetH));
                ownSource = true;
            }

            Color32[] pixels = source.GetPixels32();
            int w = source.width;
            int h = source.height;
            int hw = w * h;

            var data = new float[1 * 3 * hw];
            for (int y = 0; y < h; y++)
            {
                // Unity 的 GetPixels32 是由下往上排（row 0 在底部）。
                // 分類模型通常對上下翻轉不敏感，這裡直接依陣列順序寫入，
                // 若之後發現需要 top-down 再補翻轉。
                int rowIn = y * w;
                int rowOut = y * w;
                for (int x = 0; x < w; x++)
                {
                    Color32 p = pixels[rowIn + x];
                    data[0 * hw + rowOut + x] = p.r;
                    data[1 * hw + rowOut + x] = p.g;
                    data[2 * hw + rowOut + x] = p.b;
                }
            }

            if (ownSource) Destroy(source);

            return new Tensor<float>(new TensorShape(1, 3, h, w), data);
        }
    }
}
