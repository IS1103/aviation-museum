// WebCamDisplay.cs - 開啟攝像頭並在 Unity 內顯示即時畫面（自拍預覽）
// 使用方式：掛在 GameObject 上，指定 Display Target 為 RawImage 或 Renderer，執行後會自動開啟第一個可用鏡頭。
using System.Collections;
using UnityEngine;
using UnityEngine.UI;

namespace AirMuseum
{
    public class WebCamDisplay : MonoBehaviour
    {
        [Header("顯示目標（二選一）")]
        [Tooltip("UI 上用來顯示攝像頭畫面的 RawImage")]
        [SerializeField] private RawImage displayRawImage;
        [Tooltip("若不用 UI，可改為 3D 物體上的 Renderer（會用主紋理顯示）")]
        [SerializeField] private Renderer displayRenderer;

        [Header("選填")]
        [Tooltip("是否使用前鏡頭（手機自拍較直觀），PC 可能無前鏡頭")]
        [SerializeField] private bool preferFrontCamera = true;
        [Tooltip("是否水平鏡像（自拍時較自然）")]
        [SerializeField] private bool mirrorHorizontal = true;

        private WebCamTexture _webCamTexture;
        private bool _started;

        private void Start()
        {
            StartCoroutine(StartWebCam());
        }

        private void OnDestroy()
        {
            StopWebCam();
        }

        private void OnDisable()
        {
            StopWebCam();
        }

        private IEnumerator StartWebCam()
        {
            if (displayRawImage == null && displayRenderer == null)
            {
                Debug.LogWarning("[WebCamDisplay] 請指定 Display Raw Image 或 Display Renderer。");
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
            _started = true;
        }

        private void ApplyTexture()
        {
            if (_webCamTexture == null) return;

            Texture tex = _webCamTexture;
            if (mirrorHorizontal)
            {
                // 用 scale -1 做水平鏡像（RawImage 用 rectTransform.localScale）
                if (displayRawImage != null)
                {
                    displayRawImage.texture = tex;
                    displayRawImage.uvRect = new Rect(1, 0, -1, 1);
                    displayRawImage.color = Color.white;
                }
                if (displayRenderer != null && displayRenderer.material != null)
                {
                    displayRenderer.material.mainTexture = tex;
                    displayRenderer.material.mainTextureScale = new Vector2(-1, 1);
                    displayRenderer.material.mainTextureOffset = new Vector2(1, 0);
                }
            }
            else
            {
                if (displayRawImage != null)
                {
                    displayRawImage.texture = tex;
                    displayRawImage.uvRect = new Rect(0, 0, 1, 1);
                    displayRawImage.color = Color.white;
                }
                if (displayRenderer != null && displayRenderer.material != null)
                    displayRenderer.material.mainTexture = tex;
            }
        }

        private void Update()
        {
            // 若用 Renderer 且材質尚未設定，每幀試一次（避免 Play() 後紋理尚未就緒）
            if (_started && displayRenderer != null && displayRenderer.material != null &&
                displayRenderer.material.mainTexture != _webCamTexture)
                ApplyTexture();
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
            if (displayRenderer != null && displayRenderer.material != null)
                displayRenderer.material.mainTexture = null;
        }

        /// <summary>是否已成功開啟並顯示攝像頭。</summary>
        public bool IsActive => _started && _webCamTexture != null && _webCamTexture.isPlaying;

        /// <summary>目前使用的 WebCamTexture（可供拍照等進階使用）。</summary>
        public WebCamTexture WebCamTexture => _webCamTexture;
    }
}
