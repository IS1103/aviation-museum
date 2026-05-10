using UnityEngine;
using UnityEngine.UI;

public class Dressing : MonoBehaviour
{
    [SerializeField] private Toggle[] toggles;
    [SerializeField] private GameObject[] targets;

    private void Start()
    {
        for (int i = 0; i < toggles.Length; i++)
        {
            if (toggles[i] == null) continue;
            int idx = i;
            toggles[i].onValueChanged.AddListener(isOn => OnToggleChanged(idx, isOn));
        }

        // if (confirmButton != null)
        // {
        //     confirmButton.onClick.AddListener(OnConfirmClicked);
        // }

        ApplyCurrentSelection();
    }

    private void OnDestroy()
    {
        if (toggles != null)
        {
            for (int i = 0; i < toggles.Length; i++)
            {
                if (toggles[i] == null) continue;
                toggles[i].onValueChanged.RemoveAllListeners();
            }
        }

        // if (confirmButton != null)
        // {
        //     confirmButton.onClick.RemoveListener(OnConfirmClicked);
        // }
    }

    private void OnToggleChanged(int idx, bool isOn)
    {
        if (this == null) return;
        if (!isOn) return;
        ShowOnly(idx);
    }

    private void ApplyCurrentSelection()
    {
        int selected = -1;
        for (int i = 0; i < toggles.Length; i++)
        {
            if (toggles[i] != null && toggles[i].isOn)
            {
                selected = i;
                break;
            }
        }
        ShowOnly(selected);
    }

    private void ShowOnly(int idx)
    {
        if (targets == null) return;
        for (int i = 0; i < targets.Length; i++)
        {
            if (targets[i] == null) continue;
            targets[i].SetActive(i == idx);
        }
    }
}
