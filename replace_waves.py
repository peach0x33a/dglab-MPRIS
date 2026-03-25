import json

# Read waves
with open("DG_WAVES_V2_V3_simple.js", "r") as f:
    data = f.read()
if data.startswith("module.exports ="):
    data = data[len("module.exports ="):]
start = data.find("[")
end = data.rfind("]") + 1
waves = json.loads(data[start:end])

waveData = []
waveNames = []
for wave in waves:
    waveNames.append(wave["name"])
    json_arr = json.dumps(wave["expectedV3"])
    waveData.append(f'\t`{json_arr}`')

waves_str = "var waveData = []string{\n" + ",\n".join(waveData) + ",\n}\n"
names_str = "var waveNames = []string{" + ", ".join(f'"{name}"' for name in waveNames) + "}\n"

# Read main.go
with open("main.go", "r") as f:
    go_code = f.read()

# Find the block to replace
start_idx = go_code.find("// 预设波形数据 (参考自 socketv2)")

if start_idx != -1:
    end_idx = go_code.find("// ============================================================", start_idx)
    if end_idx != -1:
        new_go_code = go_code[:start_idx] + "// 预设波形数据 (参考自 DG_WAVES_V2_V3_simple.js)\n" + waves_str + names_str + "\n" + go_code[end_idx:]
        with open("main.go", "w") as f:
            f.write(new_go_code)
        print("Successfully replaced waveforms in main.go")
    else:
        print("Could not find end of waveform block in main.go")
else:
    print("Could not find waveform block in main.go")

