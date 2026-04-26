// PlayerInput.cs - 玩家首登輸入表單（姓名／年齡／性別）→ 呼叫 auth/validate 的 register token 完成建檔+認證。
// 注意：本類別名稱與 proto 生成的 AirMuseum.PlayerInput 同名，因此本檔不 using AirMuseum；
//        需要 AirMuseumService 請用完整命名空間 AirMuseum.AirMuseumService 引用。
using Cysharp.Threading.Tasks;
using Gate;
using UnityEngine;
using UnityEngine.UI;

public class PlayerInput : MonoBehaviour
{
    [Header("表單欄位")]
    [SerializeField] private InputField nameInput;
    [SerializeField] private InputField ageInput;

    [Header("性別 Toggle（建議掛同一個 ToggleGroup 做擇一）")]
    [Tooltip("男 → sex=0")]
    [SerializeField] private Toggle maleToggle;
    [Tooltip("女 → sex=1")]
    [SerializeField] private Toggle femaleToggle;
    [Tooltip("非二元 → sex=2")]
    [SerializeField] private Toggle nonBinaryToggle;

    [Header("確定按鈕")]
    [SerializeField] private Button submitButton;

    [Header("狀態／提示文字（選填）")]
    [SerializeField] private Text statusText;

    [Header("流程")]
    [Tooltip("送出成功後要開啟的 GameObject（同場景內切換用）")]
    [SerializeField] private GameObject nextGameObject;

    private bool _destroyed;
    private bool _subscribed;
    private bool _submitting;

    private void Awake()
    {
        AirMuseum.AirMuseumService.Instance.OnError += OnAirMuseumError;
        _subscribed = true;
    }

    private void OnDestroy()
    {
        _destroyed = true;
        if (_subscribed)
        {
            AirMuseum.AirMuseumService.Instance.OnError -= OnAirMuseumError;
            _subscribed = false;
        }
        if (submitButton != null)
            submitButton.onClick.RemoveListener(OnSubmitClicked);
    }

    private void Start()
    {
        if (submitButton != null)
            submitButton.onClick.AddListener(OnSubmitClicked);
        SetStatus("");
    }

    private void OnSubmitClicked()
    {
        if (_submitting) return;

        string name = nameInput != null ? (nameInput.text ?? "").Trim() : "";
        if (string.IsNullOrEmpty(name))
        {
            SetStatus("請輸入姓名");
            return;
        }

        int age = 0;
        string ageRaw = ageInput != null ? (ageInput.text ?? "").Trim() : "";
        if (string.IsNullOrEmpty(ageRaw) || !int.TryParse(ageRaw, out age) || age <= 0 || age > 150)
        {
            SetStatus("請輸入正確的年齡");
            return;
        }

        int sex = GetSelectedSex();

        DoRegisterAsync(name, age, sex).Forget();
    }

    // 讀 Toggle 狀態；對應關係：男=0、女=1、非二元=2。全未選退回 0。
    private int GetSelectedSex()
    {
        if (maleToggle != null && maleToggle.isOn) return 0;
        if (femaleToggle != null && femaleToggle.isOn) return 1;
        if (nonBinaryToggle != null && nonBinaryToggle.isOn) return 2;
        return 0;
    }

    private async UniTaskVoid DoRegisterAsync(string name, int age, int sex)
    {
        _submitting = true;
        SetInteractable(false);
        SetStatus("送出中…");

        var svc = AirMuseum.AirMuseumService.Instance;
        if (!svc.IsConnected)
        {
            SetStatus("尚未與伺服器連線");
            SetInteractable(true);
            _submitting = false;
            return;
        }

        string token = AirMuseum.AirMuseumService.BuildRegisterToken(name, age, sex, "player");
        var req = new ValidateReq { Token = token, GateSid = "", Device = "player" };

        ValidateResp resp = await svc.AuthAsync(req);
        if (_destroyed) return;

        if (resp == null)
        {
            SetStatus("註冊失敗，請再試一次");
            SetInteractable(true);
            _submitting = false;
            return;
        }

        uint newUid = resp.Uid;
        PlayerPrefs.SetInt("air_museum_uid", (int)newUid);
        PlayerPrefs.SetString("air_museum_name", name);
        PlayerPrefs.SetInt("air_museum_age", age);
        PlayerPrefs.SetInt("air_museum_sex", sex);
        PlayerPrefs.Save();

        Debug.Log($"[PlayerInput] 註冊成功 uid={newUid} name={name} age={age} sex={sex}");
        SetStatus($"歡迎 {name}（uid={newUid}）");

        _submitting = false;

        if (nextGameObject != null)
        {
            nextGameObject.SetActive(true);
        }
        gameObject.SetActive(false);
    }

    private void SetInteractable(bool v)
    {
        if (submitButton != null) submitButton.interactable = v;
        if (nameInput != null) nameInput.interactable = v;
        if (ageInput != null) ageInput.interactable = v;
        if (maleToggle != null) maleToggle.interactable = v;
        if (femaleToggle != null) femaleToggle.interactable = v;
        if (nonBinaryToggle != null) nonBinaryToggle.interactable = v;
    }

    private void SetStatus(string msg)
    {
        if (statusText != null) statusText.text = msg ?? "";
    }

    private void OnAirMuseumError(string msg)
    {
        if (_destroyed) return;
        SetStatus("錯誤：" + msg);
        if (_submitting)
        {
            _submitting = false;
            SetInteractable(true);
        }
    }
}
