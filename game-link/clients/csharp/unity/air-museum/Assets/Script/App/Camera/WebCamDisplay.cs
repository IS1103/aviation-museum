// WebCamDisplay.cs - 開啟攝像頭並在 Unity 內顯示即時畫面（自拍預覽）
// 使用方式：掛在 GameObject 上，指定 Display Raw Image，執行後會自動開啟第一個可用鏡頭。
using System.Collections;
using UnityEngine;
using UnityEngine.Events;
using UnityEngine.UI;

namespace AirMuseum
{
    public class WebCamDisplay : MonoBehaviour
    {
        /// <summary>確認拍照後的事件，會把拍到的 Texture2D 一併傳出。</summary>
        [System.Serializable]
        public class PhotoConfirmedEvent : UnityEvent<Texture2D> { }

        /// <summary>畫面縮放模式。</summary>
        public enum ScaleMode
        {
            /// <summary>保持比例並完整顯示畫面，容器多餘區域留空（letterbox / pillarbox）。</summary>
            Fit,
            /// <summary>保持比例並填滿容器，超出部分會被裁切。</summary>
            Fill,
            /// <summary>不保持比例，直接拉伸填滿容器（原本的行為）。</summary>
            Stretch,
        }

        [Header("顯示目標")]
        [Tooltip("UI 上用來顯示攝像頭畫面的 RawImage")]
        [SerializeField] private RawImage displayRawImage;

        [Header("顯示設定")]
        [Tooltip("畫面縮放模式：Fit 保持比例留邊、Fill 保持比例裁切、Stretch 直接拉伸")]
        [SerializeField] private ScaleMode scaleMode = ScaleMode.Fit;

        [Header("選填")]
        [Tooltip("是否使用前鏡頭（手機自拍較直觀），PC 可能無前鏡頭")]
        [SerializeField] private bool preferFrontCamera = true;
        [Tooltip("是否水平鏡像（自拍時較自然）")]
        [SerializeField] private bool mirrorHorizontal = true;

        [Header("拍照 UI")]
        [Tooltip("拍照按鈕：按下後暫停攝像頭畫面（凍結在當下這一幀），再按一次會重拍")]
        [SerializeField] private Button captureButton;
        [Tooltip("確認按鈕：按下後把凍結的畫面輸出為 Texture2D")]
        [SerializeField] private Button confirmButton;

        [Header("臉部分析（選填）")]
        [Tooltip("按下「確認」後自動執行的臉部分析元件；null 則跳過")]
        [SerializeField] private FaceAnalyzer faceAnalyzer;
        [Tooltip("每次按下「確認」時先清空，再寫入本次臉部分析（性別／年齡／眼鏡）")]
        [SerializeField] private Text logLabel;

        [Header("多國語言 Key（lan.csv）")]
        [Tooltip("拍照按鈕的翻譯 key（尚未拍照時顯示）")]
        [SerializeField] private string captureLangKey = "camera.btn_capture";
        [Tooltip("重拍按鈕的翻譯 key（已拍照後顯示在同一顆按鈕上）")]
        [SerializeField] private string retakeLangKey = "camera.btn_retake";
        [Tooltip("確認按鈕的翻譯 key")]
        [SerializeField] private string confirmLangKey = "camera.btn_confirm";

        [Header("事件")]
        [Tooltip("按下「確認」按鈕後觸發，參數為拍到的照片 Texture2D")]
        [SerializeField] private PhotoConfirmedEvent onPhotoConfirmed;

        [Header("切換下一階段")]
        [Tooltip("確認＋臉部分析完成後要開啟的下一個頁面根物件（通常是預設 inactive 的 Panel GameObject）")]
        [SerializeField] private GameObject nextStageObject;
        [Tooltip("完成後自動釋放本物件；關閉此旗標可手動控制釋放時機")]
        [SerializeField] private bool autoReleaseAfterConfirm = true;

        private WebCamTexture _webCamTexture;
        private bool _started;
        private bool _isCaptured;
        private Texture2D _capturedPhoto;

        // 按鈕上的文字（會自動從 Button 底下找 Text 元件）
        private Text _captureButtonLabel;
        private Text _confirmButtonLabel;
        private bool _languageCallbackBound;

        // RawImage 在 Editor 中設定的原始大小（作為「可用顯示區域」使用）
        private Vector2 _originalRectSize;
        private bool _originalRectSizeCached;

        // 快取上次套用的布局資訊，避免每幀重複設定 RectTransform
        private Vector2 _lastContainerSize;
        private int _lastTexWidth;
        private int _lastTexHeight;
        private float _lastRotationAngle;
        private ScaleMode _lastScaleMode;
        private bool _lastMirrorHorizontal;
        private bool _lastVerticallyMirrored;

        /// <summary>實機上 RawImage 需額外翻轉時，與 videoRotationAngle 的 Z 一併套用。</summary>
        private bool _useDeviceRawImageFlip;

        private void Awake()
        {
            _useDeviceRawImageFlip = !Application.isEditor;
        }

        private void Start()
        {
            SetupButtons();
            StartCoroutine(StartWebCam());
        }

        private void OnDestroy()
        {
            if (captureButton != null) captureButton.onClick.RemoveListener(ToggleCapture);
            if (confirmButton != null) confirmButton.onClick.RemoveListener(ConfirmPhoto);

            if (_languageCallbackBound && SetLang.Instance != null)
            {
                SetLang.Instance.OnLanguageChanged -= OnLanguageChanged;
            }
            _languageCallbackBound = false;

            if (_capturedPhoto != null)
            {
                Destroy(_capturedPhoto);
                _capturedPhoto = null;
            }

            StopWebCam();
        }

        private void OnDisable()
        {
            StopWebCam();
        }

        private void SetupButtons()
        {
            if (captureButton != null)
            {
                captureButton.onClick.RemoveListener(ToggleCapture);
                captureButton.onClick.AddListener(ToggleCapture);
                captureButton.gameObject.SetActive(true);
                _captureButtonLabel = captureButton.GetComponentInChildren<Text>(true);
            }

            if (confirmButton != null)
            {
                confirmButton.onClick.RemoveListener(ConfirmPhoto);
                confirmButton.onClick.AddListener(ConfirmPhoto);
                // 還沒拍照前先隱藏確認按鈕
                confirmButton.gameObject.SetActive(false);
                _confirmButtonLabel = confirmButton.GetComponentInChildren<Text>(true);
            }

            // 訂閱語言變更事件，切語言時自動更新按鈕文字
            if (!_languageCallbackBound && SetLang.Instance != null)
            {
                SetLang.Instance.OnLanguageChanged += OnLanguageChanged;
                _languageCallbackBound = true;
            }

            ApplyButtonLabels();
        }

        private void OnLanguageChanged(Language _)
        {
            ApplyButtonLabels();
        }

        /// <summary>依照目前的拍照狀態與語言，更新按鈕上的文字。</summary>
        private void ApplyButtonLabels()
        {
            if (_captureButtonLabel != null)
            {
                string key = _isCaptured ? retakeLangKey : captureLangKey;
                _captureButtonLabel.text = SetLang.T(key);
            }

            if (_confirmButtonLabel != null)
            {
                _confirmButtonLabel.text = SetLang.T(confirmLangKey);
            }
        }

        private IEnumerator StartWebCam()
        {
            if (displayRawImage == null)
            {
                Debug.LogWarning("[WebCamDisplay] 請指定 Display Raw Image。");
                yield break;
            }

            // 等一幀讓 Unity 權限對話框有機會出現
            yield return Application.RequestUserAuthorization(UserAuthorization.WebCam);
            if (!Application.HasUserAuthorization(UserAuthorization.WebCam))
            {
                Debug.LogWarning("[WebCamDisplay] 未取得攝像頭權限。");
                yield break;
            }

            WebCamDevice[] devices = WebCamTexture.devices;
            if (devices == null || devices.Length == 0)
            {
                Debug.LogWarning("[WebCamDisplay] 找不到任何攝像頭。");
                yield break;
            }

            WebCamDevice chosen = devices[0];
            for (int i = 0; i < devices.Length; i++)
            {
                if (devices[i].isFrontFacing == preferFrontCamera)
                {
                    chosen = devices[i];
                    break;
                }
            }

            _webCamTexture = new WebCamTexture(chosen.name);
            _webCamTexture.Play();

            // 等幾幀讓解析度穩定
            yield return null;
            yield return null;

            if (_webCamTexture == null || !_webCamTexture.didUpdateThisFrame)
            {
                int wait = 0;
                while (wait < 60 && (_webCamTexture == null || !_webCamTexture.didUpdateThisFrame))
                {
                    yield return null;
                    wait++;
                }
            }

            ApplyTexture();
            UpdateLayoutAndUV(true);
            _started = true;
        }

        private void ApplyTexture()
        {
            if (_webCamTexture == null || displayRawImage == null) return;

            displayRawImage.texture = _webCamTexture;
            displayRawImage.color = Color.white;
        }

        /// <summary>
        /// 依照 WebCamTexture 的原生寬高比，調整 RawImage 的大小與旋轉，
        /// 同時把 mirrorHorizontal 與 videoVerticallyMirrored 套用到 uvRect。
        /// </summary>
        private void UpdateLayoutAndUV(bool forceRefresh)
        {
            if (_webCamTexture == null || displayRawImage == null) return;

            int texW = _webCamTexture.width;
            int texH = _webCamTexture.height;
            // WebCamTexture 初始化後、第一次拿到影像前，width/height 會是 16（Unity 預設值），直接跳過
            if (texW <= 16 || texH <= 16) return;

            float angle = _webCamTexture.videoRotationAngle;
            bool verticallyMirrored = _webCamTexture.videoVerticallyMirrored;

            RectTransform rt = displayRawImage.rectTransform;
            RectTransform parent = rt.parent as RectTransform;

            // 第一次取得有效尺寸時，把當下 Editor 設定的 RawImage 尺寸當作「可用顯示區域」快取起來
            if (!_originalRectSizeCached)
            {
                Vector2 currentSize = rt.rect.size;
                if (currentSize.x > 0f && currentSize.y > 0f)
                {
                    _originalRectSize = currentSize;
                    _originalRectSizeCached = true;
                }
            }

            if (_originalRectSizeCached)
            {
                Vector2 containerSize = _originalRectSize;

                bool layoutChanged = forceRefresh
                    || containerSize != _lastContainerSize
                    || texW != _lastTexWidth
                    || texH != _lastTexHeight
                    || !Mathf.Approximately(angle, _lastRotationAngle)
                    || scaleMode != _lastScaleMode;

                if (layoutChanged)
                {
                    // 若鏡頭回傳的畫面被旋轉 90/270 度，顯示時的寬高需要交換
                    bool swap = Mathf.Approximately(Mathf.Abs(angle) % 180f, 90f);
                    float srcW = swap ? texH : texW;
                    float srcH = swap ? texW : texH;
                    float srcAspect = srcW / srcH;
                    float containerAspect = containerSize.x / containerSize.y;

                    float targetW, targetH;
                    switch (scaleMode)
                    {
                        case ScaleMode.Stretch:
                            targetW = containerSize.x;
                            targetH = containerSize.y;
                            break;
                        case ScaleMode.Fill:
                            if (srcAspect > containerAspect)
                            {
                                targetH = containerSize.y;
                                targetW = containerSize.y * srcAspect;
                            }
                            else
                            {
                                targetW = containerSize.x;
                                targetH = containerSize.x / srcAspect;
                            }
                            break;
                        case ScaleMode.Fit:
                        default:
                            if (srcAspect > containerAspect)
                            {
                                targetW = containerSize.x;
                                targetH = containerSize.x / srcAspect;
                            }
                            else
                            {
                                targetH = containerSize.y;
                                targetW = containerSize.y * srcAspect;
                            }
                            break;
                    }

                    // 套用旋轉（對齊 WebCamTexture.videoRotationAngle），以 pivot 為中心；實機上可額外翻轉 XY
                    rt.localEulerAngles = _useDeviceRawImageFlip
                        ? new Vector3(180f, 180f, -angle)
                        : new Vector3(0f, 0f, -angle);

                    // 若旋轉導致寬高交換，實際 rect.size 的 x/y 要以「旋轉前」的軸向為準
                    Vector2 targetSize = swap
                        ? new Vector2(targetH, targetW)
                        : new Vector2(targetW, targetH);

                    // 不動 anchorMin/Max、pivot、anchoredPosition，只改 sizeDelta
                    // rect.size = (anchorMax - anchorMin) * parentSize + sizeDelta
                    // => sizeDelta = targetSize - (anchorMax - anchorMin) * parentSize
                    Vector2 anchorSpan = rt.anchorMax - rt.anchorMin;
                    Vector2 parentSize = parent != null ? parent.rect.size : Vector2.zero;
                    rt.sizeDelta = new Vector2(
                        targetSize.x - anchorSpan.x * parentSize.x,
                        targetSize.y - anchorSpan.y * parentSize.y);

                    _lastContainerSize = containerSize;
                    _lastTexWidth = texW;
                    _lastTexHeight = texH;
                    _lastRotationAngle = angle;
                    _lastScaleMode = scaleMode;
                }
            }

            // uvRect：處理水平鏡像與垂直翻轉（行動平台有時需要）
            if (forceRefresh
                || mirrorHorizontal != _lastMirrorHorizontal
                || verticallyMirrored != _lastVerticallyMirrored)
            {
                float ux = mirrorHorizontal ? 1f : 0f;
                float uw = mirrorHorizontal ? -1f : 1f;
                float uy = verticallyMirrored ? 1f : 0f;
                float uh = verticallyMirrored ? -1f : 1f;
                displayRawImage.uvRect = new Rect(ux, uy, uw, uh);

                _lastMirrorHorizontal = mirrorHorizontal;
                _lastVerticallyMirrored = verticallyMirrored;
            }
        }

        private void LateUpdate()
        {
            if (!_started || _webCamTexture == null) return;

            // 每幀更新布局（內部有快取，只在尺寸／參數變更時才真的重設 RectTransform）
            UpdateLayoutAndUV(false);
        }

        private void StopWebCam()
        {
            if (_webCamTexture != null)
            {
                _webCamTexture.Stop();
                Destroy(_webCamTexture);
                _webCamTexture = null;
            }
            _started = false;

            if (displayRawImage != null)
            {
                displayRawImage.texture = null;
                displayRawImage.uvRect = new Rect(0, 0, 1, 1);
            }
        }

        /// <summary>是否已成功開啟並顯示攝像頭。</summary>
        public bool IsActive => _started && _webCamTexture != null && _webCamTexture.isPlaying;

        /// <summary>目前使用的 WebCamTexture（可供拍照等進階使用）。</summary>
        public WebCamTexture WebCamTexture => _webCamTexture;

        /// <summary>是否已按下拍照按鈕、畫面已凍結（尚未確認）。</summary>
        public bool IsCaptured => _isCaptured;

        /// <summary>已確認的照片（在按下「確認」後才會有值）。</summary>
        public Texture2D CapturedPhoto => _capturedPhoto;

        /// <summary>
        /// 拍照按鈕按下時的行為：
        /// - 還沒拍時：凍結畫面、顯示確認按鈕。
        /// - 已經拍了：恢復預覽（重拍）、隱藏確認按鈕。
        /// </summary>
        public void ToggleCapture()
        {
            if (_isCaptured) Retake();
            else CapturePhoto();
        }

        /// <summary>
        /// 拍照：暫停攝像頭畫面，讓顯示停在當下這一幀，並顯示確認按鈕。
        /// </summary>
        public void CapturePhoto()
        {
            if (_webCamTexture == null || !_started) return;
            if (_isCaptured) return;

            _webCamTexture.Pause();
            _isCaptured = true;

            if (confirmButton != null) confirmButton.gameObject.SetActive(true);
            ApplyButtonLabels();
        }

        /// <summary>
        /// 確認：把凍結的畫面擷取成 Texture2D，透過 onPhotoConfirmed 事件送出。
        /// </summary>
        public void ConfirmPhoto()
        {
            if (!_isCaptured || _webCamTexture == null) return;

            int w = _webCamTexture.width;
            int h = _webCamTexture.height;
            if (w <= 16 || h <= 16) return;

            if (logLabel != null)
                logLabel.text = string.Empty;

            // 釋放上一張（避免重複按造成記憶體洩漏）
            if (_capturedPhoto != null)
            {
                Destroy(_capturedPhoto);
                _capturedPhoto = null;
            }

            _capturedPhoto = new Texture2D(w, h, TextureFormat.RGB24, false);
            _capturedPhoto.SetPixels(_webCamTexture.GetPixels());
            _capturedPhoto.Apply();

            onPhotoConfirmed?.Invoke(_capturedPhoto);

            AnalyzeFaceAndLog(_capturedPhoto);

            if (autoReleaseAfterConfirm)
            {
                OpenNextStageAndRelease();
            }
        }

        /// <summary>
        /// 開啟下一階段頁面，並停止攝像頭、釋放本 GameObject。
        /// - nextStageObject 會被 SetActive(true)。
        /// - WebCamTexture 會 Stop + Destroy。
        /// - Destroy(gameObject) 會觸發 OnDestroy 釋放 _capturedPhoto / 解除事件訂閱。
        /// </summary>
        public void OpenNextStageAndRelease()
        {
            if (nextStageObject != null)
            {
                nextStageObject.SetActive(true);
            }
            else
            {
                Debug.LogWarning("[WebCamDisplay] nextStageObject 未設定，僅釋放本物件。");
            }

            StopWebCam();

            Destroy(gameObject);
        }

        /// <summary>呼叫 FaceAnalyzer 做推論，並把性別／年齡／眼鏡結果印到 Console 與 logLabel。</summary>
        private void AnalyzeFaceAndLog(Texture2D photo)
        {
            if (faceAnalyzer == null) return;

            var r = faceAnalyzer.Analyze(photo);
            if (!r.success)
            {
                const string warn = "[WebCamDisplay] 臉部分析失敗（模型未載入或輸出異常）";
                Debug.LogWarning(warn);
                if (logLabel != null)
                    logLabel.text = "臉部分析失敗（模型未載入或輸出異常）";
                return;
            }

            string glassesText = r.glassesAvailable
                ? (r.wearsGlasses ? "有戴眼鏡" : "沒戴眼鏡")
                : "未判斷（未提供眼鏡模型）";

            string genderText = r.gender == FaceAnalyzer.Gender.Male ? "男" : "女";
            string labelBlock = $"性別：{genderText}\n年齡：{r.age}\n眼鏡：{glassesText}";

            Debug.Log($"[WebCamDisplay] 臉部分析結果 → 性別: {r.gender} | 年齡: {r.age} | 眼鏡: {glassesText}");
            if (logLabel != null)
                logLabel.text = labelBlock;

            // 存入 PlayerPrefs 方便下一頁直接讀；鍵名與 Doll.cs 對齊
            // 若模型無法判斷眼鏡（glassesAvailable=false），一律視為「沒戴眼鏡」
            PlayerPrefs.SetInt("air_museum_face_gender", r.gender == FaceAnalyzer.Gender.Male ? 0 : 1);
            PlayerPrefs.SetInt("air_museum_face_age", r.age);
            PlayerPrefs.SetInt("air_museum_face_glasses", (r.glassesAvailable && r.wearsGlasses) ? 1 : 0);
            PlayerPrefs.Save();
        }

        /// <summary>
        /// 重拍：恢復攝像頭畫面，隱藏確認按鈕。
        /// </summary>
        public void Retake()
        {
            if (_webCamTexture != null && !_webCamTexture.isPlaying)
            {
                _webCamTexture.Play();
            }

            _isCaptured = false;
            if (_capturedPhoto != null)
            {
                Destroy(_capturedPhoto);
                _capturedPhoto = null;
            }

            if (captureButton != null) captureButton.gameObject.SetActive(true);
            if (confirmButton != null) confirmButton.gameObject.SetActive(false);
            ApplyButtonLabels();
        }
    }
}
