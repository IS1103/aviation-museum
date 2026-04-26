using UnityEngine;

public class Man : Role
{
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

    [Header("外觀物件")]
    [Tooltip("眼鏡的 GameObject，戴眼鏡時顯示、沒戴時隱藏")]
    [SerializeField] private GameObject glassesObj;

    void Start()
    {
        
    }

    // 依傳入狀態顯示或隱藏眼鏡物件，並同步更新 avatarGlasses（1=戴、0=沒戴）。
    public void SetGlassesVisible(bool on)
    {
        if (glassesObj != null)
        {
            glassesObj.SetActive(on);
        }
        avatarGlasses = on ? 1 : 0;
    }

    public void SetupBase(int uid, int mission, string name, int age, int sex, int gameScore, int landingScore, int ranking)
    {
        this.uid = uid;
        this.mission = mission;
        this.name = name;
        this.age = age;
        this.sex = sex;
        this.avatarGlasses = avatarGlasses;
        this.avatarHelmet = avatarHelmet;
    }

    public void SetupAppearance(int avatarGlasses, int avatarHelmet, int avatarEyes, int avatarMouth)
    {
        this.avatarGlasses = avatarGlasses;
        this.avatarHelmet = avatarHelmet;
        this.avatarEyes = avatarEyes;
        this.avatarMouth = avatarMouth;
    }
}
