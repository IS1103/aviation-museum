using UnityEngine;

public class Man : MonoBehaviour
{
    public int hp;
    public int uid;
    public int mission;
    public string name;
    public int age;
    public int sex;
    public int avatarGlasses;
    public int avatarHelmet;
    public int avatarEyes;
    public int avatarMouth;
    public int gameScore;
    public int landingScore;
    public int ranking;

    public FaceChanger eyesChanger;
    public FaceChanger eyebrowChanger;
    public FaceChanger mouthChanger;

    [Header("外觀物件")]
    [Tooltip("眼鏡變體：依索引只顯示其中一個，其餘關閉。-1（未戴）或索引無效時全部關閉。")]
    [SerializeField] private GameObject[] glassesObjects;
    [Tooltip("安全帽變體：永遠顯示其一；依索引只開啟一個款式，無效索引會自動改為 0～最後一個之間。")]
    [SerializeField] private GameObject[] helmetObjects;

    void Start()
    {
        
    }

    /// <summary>
    /// 切換眼鏡。<paramref name="idx"/> 為 <c>-1</c> 表示未戴眼鏡，會關閉 <see cref="glassesObjects"/> 內每一個；
    /// 否則只開啟第 <paramref name="idx"/> 個，其餘關閉（索引無效時同視為全部關閉，<c>avatarGlasses</c> 記為 -1）。
    /// </summary>
    public void SetGlasses(int idx)
    {
        avatarGlasses = idx;
        for (int i = 0; i < glassesObjects.Length; i++)
            glassesObjects[i].SetActive(false);

        if (idx == -1)
        {
            avatarGlasses = -1;
            WriteGlassesPlayerPrefs();
            return;
        }

        glassesObjects[idx].SetActive(true);
        WriteGlassesPlayerPrefs();
    }

    /// <summary>
    /// 切換安全帽款式（永遠會戴一頂）。<paramref name="idx"/> 會限於合法範圍，只會啟用對應一頂並同步 <c>avatarHelmet</c>。
    /// </summary>
    public void SetHelmet(int idx)
    {
        avatarHelmet = idx;

        if (helmetObjects == null || helmetObjects.Length == 0)
        {
            WriteHelmetPlayerPrefs();
            return;
        }

        idx = Mathf.Clamp(idx, 0, helmetObjects.Length - 1);
        avatarHelmet = idx;

        for (int i = 0; i < helmetObjects.Length; i++)
            helmetObjects[i].SetActive(i == idx);

        helmetObjects[idx].SetActive(true);
        WriteHelmetPlayerPrefs();
    }

    public void SetEyes(int idx){
        eyesChanger.SetFace(idx);
    }
    public void SetEyebrow(int idx){
        eyebrowChanger.SetFace(idx);
    }
    public void SetMouth(int idx){
        mouthChanger.SetFace(idx);
    }

    // 相容舊呼叫（Doll）：開啟時顯示索引 0；關閉時全部關閉且 avatarGlasses 為 -1。
    public void SetGlassesVisible(bool on)
    {
        SetGlasses(on ? 0 : -1);
    }

    public void SetupBase(int uid, int mission, string name, int age, int sex, int gameScore, int landingScore, int ranking)
    {
        this.uid = uid;
        this.mission = mission;
        this.name = name;
        this.age = age;
        this.sex = sex;
        this.gameScore = gameScore;
        this.landingScore = landingScore;
        this.ranking = ranking;
    }

    public void SetupAppearance(int avatarGlasses, int avatarHelmet, int avatarEyes, int avatarMouth)
    {
        this.avatarGlasses = avatarGlasses;
        this.avatarHelmet = avatarHelmet;
        this.avatarEyes = avatarEyes;
        this.avatarMouth = avatarMouth;
        WriteAppearanceIndicesPlayerPrefs();
    }

    /// <summary>與 <see cref="Doll"/> 相同鍵：<c>air_museum_glasses_index</c>（-1 未戴，≥0 為款式索引）。</summary>
    private void WriteGlassesPlayerPrefs()
    {
        PlayerPrefs.SetInt("air_museum_glasses_index", avatarGlasses);
        PlayerPrefs.Save();
    }

    /// <summary>與 <see cref="Doll"/> 相同鍵：<c>air_museum_helmet_index</c>。</summary>
    private void WriteHelmetPlayerPrefs()
    {
        PlayerPrefs.SetInt("air_museum_helmet_index", avatarHelmet);
        PlayerPrefs.Save();
    }

    /// <summary>寫入 <see cref="Doll"/> 使用的眼／嘴／鏡／帽索引（註冊與臉部分析鍵仍由其它流程維護）。</summary>
    private void WriteAppearanceIndicesPlayerPrefs()
    {
        PlayerPrefs.SetInt("air_museum_eyes_index", avatarEyes);
        PlayerPrefs.SetInt("air_museum_mouth_index", avatarMouth);
        PlayerPrefs.SetInt("air_museum_glasses_index", avatarGlasses);
        PlayerPrefs.SetInt("air_museum_helmet_index", avatarHelmet);
        PlayerPrefs.Save();
    }
}
