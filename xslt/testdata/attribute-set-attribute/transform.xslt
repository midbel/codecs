<?xml version="1.0" encoding="UTF-8"?>

<xsl:stylesheet version="3.0" 
	xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
	<xsl:output method="xml" indent="yes"/>
	<xsl:attribute-set name="common-attrs">
		<xsl:attribute name="type">dev</xsl:attribute>
		<xsl:attribute name="code">allowed</xsl:attribute>
		<xsl:attribute name="color">blue</xsl:attribute>
	</xsl:attribute-set>
	<xsl:template match="/">
		<root use-attribute-sets="common-attrs" type="prod" color="green">
			<xsl:attribute name="color">red</xsl:attribute>
		</root>
	</xsl:template>
</xsl:stylesheet>