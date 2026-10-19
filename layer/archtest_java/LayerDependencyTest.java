import com.tngtech.archunit.base.DescribedPredicate;
import com.tngtech.archunit.core.domain.JavaClass;
import com.tngtech.archunit.core.domain.JavaClasses;
import com.tngtech.archunit.core.domain.JavaMethodCall;
import com.tngtech.archunit.core.importer.ClassFileImporter;
import com.tngtech.archunit.core.importer.ImportOption;
import com.tngtech.archunit.library.dependencies.SliceAssignment;
import com.tngtech.archunit.library.dependencies.SliceIdentifier;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.MethodSource;
import org.springframework.beans.factory.BeanFactory;

import java.util.List;
import java.util.Objects;
import java.util.stream.IntStream;

import static com.tngtech.archunit.lang.syntax.ArchRuleDefinition.noClasses;
import static com.tngtech.archunit.library.Architectures.layeredArchitecture;
import static com.tngtech.archunit.library.dependencies.SlicesRuleDefinition.slices;

/**
 * 入口层 -> service -> infra，dto/enums/utils 为公共包，config 为组装。
 */
class LayerDependencyTest {

    private static final String ROOT = "com.test.strategy";

    // 静态初始化而非 @BeforeAll，因为 packageDepths 参数源也要用到
    private static final JavaClasses classes = new ClassFileImporter()
            .withImportOption(ImportOption.Predefined.DO_NOT_INCLUDE_TESTS)
            .importPackages(ROOT);

    /**
     * 入口层只能被组装访问，service 只能被入口层访问，infra 只能被 service 访问，公共包不能依赖任何层。
     * 所有类都必须归属某一层，新增顶层包时需在此登记；包用绝对路径，避免子包同名被误归层。
     * 根包下的启动类、生命周期类归入组装层。
     */
    @Test
    void layers() {
        layeredArchitecture().consideringOnlyDependenciesInLayers()
                .ensureAllClassesAreContainedInArchitecture()
                .layer("Assembly").definedBy(ROOT, ROOT + ".config..", ROOT + ".schedule..")
                .layer("Entry").definedBy(ROOT + ".controller..")
                .layer("Service").definedBy(ROOT + ".service..")
                .layer("Infra").definedBy(ROOT + ".infra..", ROOT + ".websocket..")
                .layer("Common").definedBy(ROOT + ".dto..", ROOT + ".enums..", ROOT + ".utils..")
                .whereLayer("Assembly").mayNotBeAccessedByAnyLayer()
                .whereLayer("Entry").mayOnlyBeAccessedByLayers("Assembly")
                .whereLayer("Service").mayOnlyBeAccessedByLayers("Entry", "Assembly")
                .whereLayer("Infra").mayOnlyBeAccessedByLayers("Service", "Assembly")
                .whereLayer("Common").mayOnlyBeAccessedByLayers("Entry", "Service", "Infra", "Assembly")
                .check(classes);
    }

    /**
     * 按包层级逐层检查循环依赖：每一层把包名截断到该深度作为切片，比该深度浅的包保留原名。
     * 深度 1 检查顶层包之间，最深一层检查任意两个包之间，中间层检查各级子模块之间。
     * 逐层检查是因为子包之间无环不代表父包之间无环，例如 a.x -> b.y 且 b.z -> a.w。
     * 深度从 1 到工程中最深的包，由 packageDepths 自动计算。
     */
    @ParameterizedTest
    @MethodSource("packageDepths")
    void noPackageCyclesAtEachLevel(int depth) {
        slices().assignedFrom(new SliceAssignment() {
                    @Override
                    public SliceIdentifier getIdentifierOf(JavaClass javaClass) {
                        List<String> parts = relativePackages(javaClass);
                        if (parts == null) {
                            return SliceIdentifier.ignore();
                        }
                        List<String> kept = parts.subList(0, Math.min(depth, parts.size()));
                        return SliceIdentifier.of(kept.isEmpty() ? ROOT : ROOT + "." + String.join(".", kept));
                    }

                    @Override
                    public String getDescription() {
                        return "截断到第 " + depth + " 层的包";
                    }
                })
                .should().beFreeOfCycles()
                .check(classes);
    }

    static IntStream packageDepths() {
        int maxDepth = classes.stream()
                .map(LayerDependencyTest::relativePackages)
                .filter(Objects::nonNull)
                .mapToInt(List::size)
                .max()
                .orElse(0);
        return IntStream.rangeClosed(1, maxDepth);
    }

    /**
     * 类所在包相对 ROOT 的各级包名，ROOT 本身为空列表，不在 ROOT 下返回 null。
     */
    private static List<String> relativePackages(JavaClass javaClass) {
        String pkg = javaClass.getPackageName();
        if (pkg.equals(ROOT)) {
            return List.of();
        }
        if (!pkg.startsWith(ROOT + ".")) {
            return null;
        }
        return List.of(pkg.substring(ROOT.length() + 1).split("\\."));
    }

    /**
     * 禁止通过容器获取 Bean 绕过依赖规则，依赖一律通过注入获得。
     */
    @Test
    void noGetBean() {
        noClasses()
                .that().doNotHaveFullyQualifiedName(AopContextUtils.class.getName())
                .should().callMethodWhere(DescribedPredicate.describe("调用 getBean 获取 Bean",
                        (JavaMethodCall call) -> call.getName().equals("getBean")
                                && (call.getTargetOwner().isAssignableTo(BeanFactory.class)
                                || call.getTargetOwner().isEquivalentTo(AopContextUtils.class))))
                .check(classes);
    }

    /**
     * service 不直接使用 Redis、Kafka、MyBatis 等中间件 API，必须通过 infra 访问。
     */
    @Test
    void serviceNotUseMiddleware() {
        noClasses()
                .that().resideInAPackage(ROOT + ".service..")
                .should().dependOnClassesThat().resideInAnyPackage(
                        "io.lettuce..",
                        "org.redisson..",
                        "org.springframework.data.redis..",
                        "org.springframework.kafka..",
                        "org.apache.kafka..",
                        "com.baomidou..")
                .check(classes);
    }
}
