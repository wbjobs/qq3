package services

import (
	"strings"
	"unicode"
)

type TranslateService struct{}

func NewTranslateService() *TranslateService {
	return &TranslateService{}
}

func (s *TranslateService) ContainsChinese(text string) bool {
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func (s *TranslateService) NeedsTranslation(text string) bool {
	if strings.TrimSpace(text) == "" {
		return false
	}
	return !s.ContainsChinese(text)
}

func (s *TranslateService) TranslateToChinese(text string) (string, error) {
	if !s.NeedsTranslation(text) {
		return "", nil
	}
	return s.mockTranslateAPI(text)
}

func (s *TranslateService) mockTranslateAPI(text string) (string, error) {
	dict := map[string]string{
		"hello":              "你好",
		"world":              "世界",
		"hello world":        "你好，世界",
		"good morning":       "早上好",
		"good afternoon":     "下午好",
		"good evening":       "晚上好",
		"good night":         "晚安",
		"thank you":          "谢谢",
		"thanks":             "谢谢",
		"please":             "请",
		"yes":                "是的",
		"no":                 "不",
		"ok":                 "好的",
		"okay":               "好的",
		"how are you":        "你好吗",
		"i love you":         "我爱你",
		"goodbye":            "再见",
		"bye":                "再见",
		"welcome":            "欢迎",
		"sorry":              "对不起",
		"excuse me":          "打扰一下",
		"what is your name":  "你叫什么名字",
		"my name is":         "我的名字是",
		"nice to meet you":   "很高兴认识你",
		"have a good day":    "祝你有美好的一天",
		"see you later":      "待会儿见",
		"i don't know":       "我不知道",
		"i understand":       "我明白了",
		"i don't understand": "我不明白",
		"can you help me":    "你能帮我吗",
		"where is":           "在哪里",
		"how much":           "多少钱",
		"i'm hungry":         "我饿了",
		"i'm tired":          "我累了",
		"let's go":           "我们走吧",
		"wait a minute":      "等一下",
		"be careful":         "小心",
		"congratulations":    "恭喜",
		"happy birthday":     "生日快乐",
		"merry christmas":    "圣诞快乐",
		"happy new year":     "新年快乐",
		"copy":               "复制",
		"paste":              "粘贴",
		"clipboard":          "剪贴板",
		"sync":               "同步",
		"message":            "消息",
		"login":              "登录",
		"logout":             "登出",
		"register":           "注册",
		"username":           "用户名",
		"password":           "密码",
		"email":              "邮箱",
		"device":             "设备",
		"phone":              "手机",
		"computer":           "电脑",
		"web":                "网页",
		"mobile":             "移动端",
		"translation":        "翻译",
		"original":           "原文",
		"history":            "历史",
		"delete":             "删除",
		"refresh":            "刷新",
		"success":            "成功",
		"error":              "错误",
		"warning":            "警告",
		"info":               "信息",
		"loading":            "加载中",
		"cancel":             "取消",
		"confirm":            "确认",
		"save":               "保存",
		"edit":               "编辑",
		"close":              "关闭",
		"open":               "打开",
		"search":             "搜索",
		"settings":           "设置",
	}

	lowerText := strings.ToLower(strings.TrimSpace(text))
	if val, ok := dict[lowerText]; ok {
		return val, nil
	}

	words := strings.Fields(lowerText)
	var translatedWords []string
	for _, word := range words {
		word = strings.Trim(word, ".,!?;:()[]{}'\"")
		if val, ok := dict[word]; ok {
			translatedWords = append(translatedWords, val)
		} else {
			translatedWords = append(translatedWords, word)
		}
	}

	if len(translatedWords) > 0 {
		result := strings.Join(translatedWords, "")
		if result == lowerText || result == strings.Join(words, "") {
			return "[模拟翻译] " + text, nil
		}
		return result, nil
	}

	return "[模拟翻译] " + text, nil
}

func (s *TranslateService) ProcessClipboardContent(content string) (translation string, needsTrans bool, err error) {
	needsTrans = s.NeedsTranslation(content)
	if !needsTrans {
		return "", false, nil
	}
	translation, err = s.TranslateToChinese(content)
	return translation, true, err
}
