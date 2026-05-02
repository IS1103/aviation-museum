using UnityEngine;
using UnityEngine.InputSystem;
using UnityEngine.UI;

public class FaceChanger : MonoBehaviour
{
    public Renderer targetRenderer;
    public int gridSize = 2;
    public Button[] faceButtons;

    private MaterialPropertyBlock block;

    void Start()
    {
        block = new MaterialPropertyBlock();
        SetFace(0);

        for (int i = 0; i < faceButtons.Length; i++)
        {
            int index = i;
            if (faceButtons[i] != null)
            {
                faceButtons[i].onClick.AddListener(() => SetFace(index));
            }
        }
    }

    void Update()
    {
        // Keyboard kb = Keyboard.current;
        // if (kb == null) return;

        // if (kb.qKey.wasPressedThisFrame) SetFace(0);
        // if (kb.wKey.wasPressedThisFrame) SetFace(1);
        // if (kb.eKey.wasPressedThisFrame) SetFace(2);
        // if (kb.rKey.wasPressedThisFrame) SetFace(3);
    }

    public void SetFace(int index)
    {
        Debug.Log("SetFace: " + index);

        int x = index % gridSize;
        int y = index / gridSize;

        Vector2 scale = new Vector2(1f, 1f);
        float size = 1f / gridSize;
        Vector2 offset = new Vector2(x * size, y * size);

        targetRenderer.GetPropertyBlock(block);
        block.SetVector("_MainTex_ST", new Vector4(scale.x, scale.y, offset.x, offset.y));
        targetRenderer.SetPropertyBlock(block);
    }

    // ✅ 新增的方法
    void Test()
    {
    }
}