import os
import shutil


def _camel_to_snake(camel_str, split='_'):
    """
    将大驼峰命名字符串转换为下划线分隔的小写字符串
    例如: MyClassName -> my_class_name
    """
    if not camel_str:
        return ""

    snake_str = [camel_str[0].lower()]
    for char in camel_str[1:]:
        if char.isupper():
            snake_str.extend([split, char.lower()])
        else:
            snake_str.append(char)
    return ''.join(snake_str)


def write_to_file(file_path, content, mode='w', encoding='utf-8'):
    """
    安全写入内容到指定文件
    :param file_path: 文件路径
    :param content: 要写入的内容
    :param mode: 写入模式 'w'覆盖写入/'a'追加写入
    :param encoding: 文件编码
    :return: bool 是否成功
    """
    try:
        with open(file_path, mode, encoding=encoding) as f:
            f.write(content)
        return True
    except PermissionError:
        print(f"错误：没有写入权限 {file_path}")
    except FileNotFoundError:
        print(f"错误：文件路径不存在 {file_path}")
    except Exception as e:
        print(f"写入文件出错: {str(e)}")
    return False


def copy_file_with_suffix(src_path, suffix=".tmp") -> str:
    """
    复制文件并添加后缀
    :param src_path: 源文件路径
    :param suffix: 新文件后缀
    :return: 新文件路径
    """
    if not os.path.exists(src_path):
        raise FileNotFoundError(f"源文件不存在: {src_path}")

    if not os.path.isfile(src_path):
        raise ValueError(f"路径不是文件: {src_path}")

    base, ext = os.path.splitext(src_path)
    dest_path = f"{base}{suffix}{ext}"

    try:
        shutil.copy2(src_path, dest_path)
        return dest_path
    except Exception as e:
        raise RuntimeError(f"复制失败: {str(e)}")


def _replace_content(file_path: str, replacer: dict[str, str]) -> None:
    """处理单个文件内容"""
    # 创建临时文件
    temp_path = copy_file_with_suffix(file_path)
    try:
        modified = False
        with open(file_path, 'r', encoding='utf-8') as src, \
                open(temp_path, 'w', encoding='utf-8') as dest:
            for line in src:
                new_line = line
                for key, value in replacer.items():
                    new_line = new_line.replace(key, value)

                if new_line != line:
                    modified = True
                    dest.write(new_line)
                else:
                    dest.write(line)

        # 替换原文件
        if modified:
            os.replace(temp_path, file_path)
            print(f"已修改: {file_path}")
            return

        # 无修改则删除临时文件
        print(f"未修改: {file_path}")
        if os.path.exists(temp_path):
            os.remove(temp_path)
        return

    except UnicodeDecodeError:
        print(f"跳过二进制文件: {file_path}")
        if os.path.exists(temp_path):
            os.remove(temp_path)
        return
    except Exception as e:
        print(f"处理文件 {file_path} 出错: {str(e)}")
        if os.path.exists(temp_path):
            os.remove(temp_path)
        return


def _replace_filenames(dir_path: str, dir_replacer: dict[str, str], file_replacer: dict[str, str]) -> list[str]:
    """递归处理目录文件"""
    file_paths = []

    for root, dirs, files in os.walk(dir_path):
        # 处理目录名
        for src_dir_name in dirs[:]:  # 使用副本遍历
            for dir_key, dir_value in dir_replacer.items():
                if dir_key in src_dir_name:
                    old_path = os.path.join(root, src_dir_name)
                    new_path = os.path.join(root, src_dir_name.replace(dir_key, dir_value))
                    os.rename(old_path, new_path)
                    dirs[dirs.index(src_dir_name)] = new_path.split(os.sep)[-1]

        # 处理文件名
        for src_file_name in files:
            file_path = os.path.join(root, src_file_name)
            src_dir_name, src_file_name = os.path.split(file_path)

            for file_key, file_value in file_replacer.items():
                if file_key in src_file_name:
                    new_path = os.path.join(src_dir_name, src_file_name.replace(file_key, file_value))
                    os.rename(file_path, new_path)
                    file_path = new_path

                if file_path not in file_paths:
                    file_paths.append(file_path)

    return file_paths


class MultiModuleGenerator:
    def __init__(self, template_path: str):
        self.template_path = template_path
        self.template_name = "HelloWorld"
        self.template_module_name = "Module"
        self.template_go_mod = "github.com/FlyTOmeLight/server-go/examples/example-hello-world"

    def generate(self, name: str, output_dir: str, modules: list[str], go_mod: str):
        try:
            os.makedirs(output_dir, exist_ok=True)

            # 创建临时工作目录
            project_dir = os.path.join(output_dir, _camel_to_snake(name, "-"))
            temp_dir = f"{project_dir}_temp"
            if os.path.exists(temp_dir):
                shutil.rmtree(temp_dir)

            # 复制模板
            shutil.copytree(self.template_path, temp_dir)

            try:
                self._process_project(temp_dir, name, go_mod)
            except Exception as e:
                print(f"项目处理失败: {str(e)}")
                return False

            try:
                self._process_modules(temp_dir, modules)
            except Exception as e:
                print(f"模块处理失败: {str(e)}")
                return False

            # 重命名为最终项目
            if os.path.exists(project_dir):
                shutil.rmtree(project_dir)
            os.rename(temp_dir, project_dir)
            print("生成成功")

        except Exception as e:
            print(f"生成失败: {str(e)}")
            return False

    def _process_project(self, file_path: str, name: str, go_mod: str):
        print(f"start process project file_path: {file_path}, name: {name}, go_mod: {go_mod}")
        os.remove(f"{file_path}/go.sum")
        # 重写go.mod
        go_mod_content = (f"module {go_mod}\n\ngo 1.24\n\nrequire (\n"
                          f"   github.com/FlyTOmeLight/server-go v0.1.0\n"
                          f"   github.com/jinzhu/copier v0.4.0\n"
                          f"   github.com/spf13/cobra v1.7.0\n"
                          f"   github.com/swaggo/swag v1.8.1\n)")
        write_to_file(f"{file_path}/go.mod", go_mod_content)

        replacer = {
            self.template_name.lower(): name.lower(),
            _camel_to_snake(self.template_name): _camel_to_snake(name),
            _camel_to_snake(self.template_name, "-"): _camel_to_snake(name, "-"),
        }
        file_paths = _replace_filenames(file_path, replacer, replacer)

        for path in file_paths:
            _replace_content(path, {
                self.template_go_mod: go_mod,
                self.template_name: name,
                self.template_name.lower(): name.lower(),
            })
        print(f"end process project")


    def _process_modules(self, file_path: str, modules: list[str]):
        print(f"start process modules file_path: {file_path}, modules: {modules}")

        deleted = []

        # internal/app/module -> internal/app/{custom module}
        for root, dirs, files in os.walk(file_path):
            for src_dir_name in dirs[:]:  # 使用副本遍历
                if self.template_module_name.lower() in src_dir_name:
                    old_path = os.path.join(root, src_dir_name)
                    if old_path not in deleted:
                        deleted.append(old_path)

                    for module_name in modules:
                        new_path = os.path.join(root, src_dir_name.replace(self.template_module_name.lower(),
                                                                           module_name.lower()))
                        shutil.copytree(old_path, new_path)

                        for new_root, _, new_files in os.walk(new_path):
                            for new_file_name in new_files:
                                if _camel_to_snake(self.template_module_name) not in new_file_name:
                                    new_file_path = os.path.join(new_root, new_file_name)
                                    print(f"ReplaceFilePath: {new_file_path}")
                                    self._process_module_file_content(new_file_path, module_name)

            for src_file_name in files:
                file_path = os.path.join(root, src_file_name)
                if _camel_to_snake(self.template_module_name) in src_file_name:
                    src_dir_name, src_file_name = os.path.split(file_path)

                    if file_path not in deleted:
                        deleted.append(file_path)

                    for module_name in modules:
                        new_name = src_file_name.replace(_camel_to_snake(self.template_module_name),
                                                         _camel_to_snake(module_name))
                        new_path = os.path.join(src_dir_name, new_name)
                        shutil.copy2(file_path, new_path)
                        self._process_module_file_content(new_path, module_name)
                    continue

                if file_path.endswith(".mod") or file_path.endswith("server/server.go"):
                    continue
                self._multi_replace_modules_content(file_path, modules)

        for deleted_path in deleted:
            if not os.path.exists(deleted_path):
                continue
            if os.path.isfile(deleted_path):
                os.remove(deleted_path)
                continue
            shutil.rmtree(deleted_path)

        print("end process modules")


    def _process_module_file_content(self, file_path: str, module_name: str):
        _replace_content(file_path, {
            _camel_to_snake(self.template_module_name): _camel_to_snake(module_name),
            self.template_module_name: module_name,
            self.template_module_name.lower(): module_name.lower(),
        })

    def _multi_replace_modules_content(self, file_path: str, modules: list[str]):
        # 创建临时文件
        temp_path = copy_file_with_suffix(file_path)
        try:
            modified = False
            name = self.template_module_name
            with open(file_path, 'r', encoding='utf-8') as src, \
                    open(temp_path, 'w', encoding='utf-8') as dest:
                for line in src:
                    edited = False
                    if name in line or name.lower() in line or _camel_to_snake(name) in line:
                        edited = True

                    for module in modules:
                        if module in line or module.lower() in line or _camel_to_snake(module) in line:
                            edited = False

                    if not edited:
                        dest.write(line)
                        continue

                    modified = True
                    for module in modules:
                        new_line = line.replace(name, module)
                        new_line = new_line.replace(name.lower(), module.lower())
                        dest.write(new_line)

            # 替换原文件
            if modified:
                os.replace(temp_path, file_path)
                print(f"已修改: {file_path}")
                return

            # 无修改则删除临时文件
            if os.path.exists(temp_path):
                os.remove(temp_path)
            return

        except UnicodeDecodeError:
            print(f"跳过二进制文件: {file_path}")
            if os.path.exists(temp_path):
                os.remove(temp_path)
            return
        except Exception as e:
            print(f"处理文件 {file_path} 出错: {str(e)}")
            if os.path.exists(temp_path):
                os.remove(temp_path)
            return


if __name__ == "__main__":
    import argparse

    parser = argparse.ArgumentParser(description='以server-go/examples/example-hello-world为模板批量生成项目代码')
    parser.add_argument('--template', default='./examples/example-hello-world', help='模板目录路径，默认为example-hello-world的路径')
    parser.add_argument('--output', default='../output', help='输出目录路径，生成后的项目代码会在该路径下')
    parser.add_argument('--name', required=True, help='项目名')
    parser.add_argument('--modules', nargs='+', default=['Module'], help='模块名称列表')
    parser.add_argument('--go-mod', required=True, help='go mod地址，例如：github.com/FlyTOmeLight/server-go')

    args = parser.parse_args()

    generator = MultiModuleGenerator(args.template)
    generator.generate(args.name, args.output, args.modules, args.go_mod)


# examples:
# python scripts/gen_code.py --name Temp --modules ModuleA ModuleB --go-mod 'github.com/your-org/temp-service'
# 如果需要指定输出目录：
# python scripts/gen_code.py --output '../temp-service' --name Temp --modules ModuleA ModuleB --go-mod 'github.com/your-org/temp-service'
# 生成完后，在新代码库根目录下 go mod tidy，即可使用