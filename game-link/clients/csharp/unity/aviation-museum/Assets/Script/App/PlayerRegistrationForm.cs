// PlayerRegistrationForm.cs - 玩家姓名／年齡／性別表單 → 寫入 PlayerPrefs，並以 SAVE_APPEARANCE 同步已建檔之 uid（須先完成 auth/validate）。
using AirMuseum;
using Cysharp.Threading.Tasks;
using UnityEngine;
using UnityEngine.UI;

public class PlayerRegistrationForm : MonoBehaviour
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

    [Header("性別 Label（被選擇時改成 selectedLabelColor，否則還原）")]
    [SerializeField] private Text maleLabel;
    [SerializeField] private Text femaleLabel;
    [SerializeField] private Color selectedLabelColor = Color.black;

    private Color _maleOriginalColor;
    private Color _femaleOriginalColor;

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

        if (maleLabel != null) _maleOriginalColor = maleLabel.color;
        if (femaleLabel != null) _femaleOriginalColor = femaleLabel.color;
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

        if (maleToggle != null) maleToggle.onValueChanged.RemoveListener(OnMaleToggleChanged);
        if (femaleToggle != null) femaleToggle.onValueChanged.RemoveListener(OnFemaleToggleChanged);
    }

    private void Start()
    {
        if (submitButton != null)
            submitButton.onClick.AddListener(OnSubmitClicked);

        if (maleToggle != null)
        {
            maleToggle.onValueChanged.AddListener(OnMaleToggleChanged);
            ApplyLabelColor(maleLabel, _maleOriginalColor, maleToggle.isOn);
        }
        if (femaleToggle != null)
        {
            femaleToggle.onValueChanged.AddListener(OnFemaleToggleChanged);
            ApplyLabelColor(femaleLabel, _femaleOriginalColor, femaleToggle.isOn);
        }

        SetStatus("");
    }

    private void OnMaleToggleChanged(bool isOn) => ApplyLabelColor(maleLabel, _maleOriginalColor, isOn);
    private void OnFemaleToggleChanged(bool isOn) => ApplyLabelColor(femaleLabel, _femaleOriginalColor, isOn);

    private void ApplyLabelColor(Text label, Color originalColor, bool isOn)
    {
        if (label == null) return;
        label.color = isOn ? selectedLabelColor : originalColor;
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

        int sessionUid = PlayerPrefs.GetInt("air_museum_uid", 0);
        if (sessionUid <= 0)
        {
            SetStatus("請先完成連線認證（無有效 session uid）");
            SetInteractable(true);
            _submitting = false;
            return;
        }

        PlayerPrefs.SetString("air_museum_name", name);
        PlayerPrefs.SetInt("air_museum_age", age);
        PlayerPrefs.SetInt("air_museum_sex", sex);
        PlayerPrefs.Save();

        int ageInt = Mathf.Clamp(age, 0, 150);
        svc.SendPlayer(new PlayerInput
        {
            Action = Action.SaveAppearance,
            Name = name,
            Age = (uint)ageInt,
            Sex = sex,
            AvatarEyes = PlayerPrefs.GetInt("air_museum_eyes_index", 0),
            AvatarEyebrow = PlayerPrefs.GetInt("air_museum_eyebrow_index", 0),
            AvatarMouth = PlayerPrefs.GetInt("air_museum_mouth_index", 0),
            AvatarGlasses = PlayerPrefs.GetInt("air_museum_glasses_index", -1),
            AvatarHelmet = PlayerPrefs.GetInt("air_museum_helmet_index", 0),
        });

        Debug.Log($"[PlayerRegistrationForm] 已送出名冊與裝扮索引 uid={sessionUid} name={name}");
        SetStatus($"歡迎 {name}");

        _submitting = false;
        SetInteractable(true);

        if (nextGameObject != null)
            nextGameObject.SetActive(true);
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